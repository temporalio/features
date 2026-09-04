use anyhow::{Context, Result, anyhow, bail, ensure};
pub use async_trait::async_trait;
use serde::Serialize;
use std::{
    env,
    error::Error,
    fmt::{self, Display, Formatter},
    fs::{self, File},
    future::Future,
    io::{BufWriter, Write},
    net::TcpStream,
    sync::mpsc,
    thread,
    time::{Duration, Instant},
};
use temporalio_client::{
    Client, ClientOptions, ClientTlsOptions, Connection, ConnectionOptions,
    HttpConnectProxyOptions, TlsOptions, WorkflowHandle, WorkflowStartOptions,
    grpc::WorkflowService,
    tonic::{Code, Request},
};
use temporalio_common::{
    HasWorkflowDefinition,
    protos::temporal::api::{
        common::v1::{Payload, WorkflowExecution},
        enums::v1::{EventType, UpdateWorkflowExecutionLifecycleStage},
        history::v1::HistoryEvent,
        update::v1::{Input, Meta, Request as UpdateRequest, WaitPolicy},
        workflowservice::v1::{ListWorkerDeploymentsRequest, UpdateWorkflowExecutionRequest},
    },
};
use temporalio_sdk::{Runtime, Worker, WorkerOptions, runtime::RuntimeOptions};
use temporalio_sdk_core::Url;
use tokio::sync::oneshot;
use uuid::Uuid;

/// Context supplied to a feature's execution.
pub struct FeatureContext {
    /// Client connected to the requested server and namespace.
    pub client: Client,
    /// Task queue assigned to this feature run.
    pub task_queue: String,
    /// Unique workflow ID suitable for the feature's default execution.
    pub workflow_id: String,
}

impl FeatureContext {
    /// Create start options for this feature's default workflow execution.
    pub fn workflow_start_options(&self) -> WorkflowStartOptions {
        WorkflowStartOptions::new(self.task_queue.clone(), self.workflow_id.clone())
            .execution_timeout(Duration::from_secs(60))
            .build()
    }
}

/// How an auxiliary worker should stop after it receives a shutdown request.
#[derive(Clone, Copy)]
pub enum WorkerShutdownMode {
    /// Wait for the worker's run future to complete after requesting shutdown.
    Graceful,
    /// Drop the worker's run future after requesting shutdown.
    Immediate,
}

/// A worker running on its own Tokio runtime for features that need to control its lifecycle.
pub struct RunningWorker {
    stop: oneshot::Sender<()>,
    thread: thread::JoinHandle<Result<()>>,
}

impl RunningWorker {
    /// Start an auxiliary worker and wait until its runtime has been initialized.
    pub fn start(
        client: Client,
        options: WorkerOptions,
        shutdown_mode: WorkerShutdownMode,
    ) -> Result<Self> {
        let (stop, stop_receiver) = oneshot::channel();
        let (ready, ready_receiver) = mpsc::sync_channel(0);
        let thread = thread::spawn(move || {
            let tokio_runtime = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()?;
            tokio_runtime.block_on(async move {
                let runtime = Runtime::new_assume_tokio(RuntimeOptions::default())?;
                let mut worker = Worker::new(&runtime, client, options)
                    .map_err(|error| anyhow!(error.to_string()))?;
                let shutdown = worker.shutdown_handle();
                ready
                    .send(())
                    .map_err(|_| anyhow!("worker starter was dropped"))?;
                let mut run = Box::pin(worker.run());
                tokio::select! {
                    result = &mut run => result.map_err(|error| anyhow!(error.to_string())),
                    _ = stop_receiver => {
                        shutdown();
                        match shutdown_mode {
                            WorkerShutdownMode::Graceful => {
                                run.await.map_err(|error| anyhow!(error.to_string()))
                            }
                            WorkerShutdownMode::Immediate => Ok(()),
                        }
                    }
                }
            })
        });
        ready_receiver
            .recv_timeout(Duration::from_secs(10))
            .map_err(|error| anyhow!("worker did not start: {error}"))?;
        Ok(Self { stop, thread })
    }

    /// Request shutdown and wait for the worker thread. Returns false if it did not stop in time.
    pub async fn stop(self, timeout: Duration) -> Result<bool> {
        let Self { stop, thread } = self;
        let _ = stop.send(());
        let deadline = Instant::now() + timeout;
        while !thread.is_finished() && Instant::now() < deadline {
            tokio::time::sleep(Duration::from_millis(50)).await;
        }
        if thread.is_finished() {
            thread
                .join()
                .map_err(|_| anyhow!("worker thread panicked"))??;
            Ok(true)
        } else {
            Ok(false)
        }
    }
}

/// Poll an asynchronous operation until it returns a value or the timeout expires.
pub async fn poll_until<T, F, Fut>(
    timeout: Duration,
    interval: Duration,
    description: &str,
    mut poll: F,
) -> Result<T>
where
    F: FnMut() -> Fut,
    Fut: Future<Output = Result<Option<T>>>,
{
    let deadline = Instant::now() + timeout;
    loop {
        if let Some(value) = poll().await? {
            return Ok(value);
        }
        ensure!(Instant::now() < deadline, "{description}");
        tokio::time::sleep(interval).await;
    }
}

/// Poll a workflow history until `find` returns a value or the timeout expires.
pub async fn wait_for_history<W, T>(
    handle: &WorkflowHandle<Client, W>,
    timeout: Duration,
    description: &str,
    mut find: impl FnMut(&[HistoryEvent]) -> Option<T>,
) -> Result<T>
where
    W: HasWorkflowDefinition,
{
    let deadline = Instant::now() + timeout;
    loop {
        let history = handle
            .fetch_history(Default::default())
            .into_events()
            .await?;
        if let Some(value) = find(&history) {
            return Ok(value);
        }
        ensure!(Instant::now() < deadline, "{description}");
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

/// Poll a workflow history until it contains an event of the requested type.
pub async fn wait_for_history_event<W>(
    handle: &WorkflowHandle<Client, W>,
    event_type: EventType,
    timeout: Duration,
    description: &str,
) -> Result<()>
where
    W: HasWorkflowDefinition,
{
    wait_for_history(handle, timeout, description, |events| {
        history_has_event(events, event_type).then_some(())
    })
    .await
}

/// Return whether a workflow history contains an event of the requested type.
pub fn history_has_event(history: &[HistoryEvent], event_type: EventType) -> bool {
    history.iter().any(|event| event.event_type() == event_type)
}

/// Count events of the requested type in a workflow history.
pub fn history_event_count(history: &[HistoryEvent], event_type: EventType) -> usize {
    history
        .iter()
        .filter(|event| event.event_type() == event_type)
        .count()
}

/// Return the first payload passed to a workflow in its start event.
pub fn workflow_input_payload(history: &[HistoryEvent]) -> Result<&Payload> {
    history
        .iter()
        .find_map(|event| match event.attributes.as_ref() {
            Some(
                temporalio_common::protos::temporal::api::history::v1::history_event::Attributes::WorkflowExecutionStartedEventAttributes(attributes),
            ) => attributes.input.as_ref()?.payloads.first(),
            _ => None,
        })
        .ok_or_else(|| anyhow!("workflow argument payload not found"))
}

/// Return the first payload produced by a completed workflow.
pub fn workflow_result_payload(history: &[HistoryEvent]) -> Result<&Payload> {
    history
        .iter()
        .find_map(|event| match event.attributes.as_ref() {
            Some(
                temporalio_common::protos::temporal::api::history::v1::history_event::Attributes::WorkflowExecutionCompletedEventAttributes(attributes),
            ) => attributes.result.as_ref()?.payloads.first(),
            _ => None,
        })
        .ok_or_else(|| anyhow!("workflow result payload not found"))
}

/// Ensure a workflow's first input and result payloads match, returning the result payload.
pub fn ensure_workflow_input_matches_result(history: &[HistoryEvent]) -> Result<&Payload> {
    let input = workflow_input_payload(history)?;
    let result = workflow_result_payload(history)?;
    ensure!(input == result, "workflow input and result payloads differ");
    Ok(result)
}

/// A server capability that a feature requires before its worker is started.
#[derive(Clone, Copy)]
pub enum ServerCapability {
    /// Workflow updates that wait through the completed lifecycle stage.
    Update,
    /// Workflow updates that return after the accepted lifecycle stage.
    AsyncAcceptedUpdate,
    /// Worker deployment versioning APIs.
    Deployment,
}

/// A Rust SDK feature that can configure a worker and execute its assertions.
#[async_trait]
pub trait Feature: Send + Sync {
    /// Add this feature's configuration to client options prepared by the harness.
    fn client_options(&self, client_options: ClientOptions) -> Result<ClientOptions> {
        Ok(client_options)
    }

    /// Add this feature's configuration to worker options prepared by the harness.
    fn worker_options(&self, worker_options: WorkerOptions) -> Result<WorkerOptions> {
        ensure!(
            !self.uses_worker(),
            "feature that uses a worker must configure its worker options"
        );
        Ok(worker_options)
    }

    /// Whether the harness should run a worker for this feature.
    fn uses_worker(&self) -> bool {
        true
    }

    /// Server capability required by this feature, if any.
    fn required_server_capability(&self) -> Option<ServerCapability> {
        None
    }

    /// Execute the feature and assert its result.
    async fn execute(&self, context: FeatureContext) -> Result<()>;
}

/// Run registered features using command-line arguments from the Go runner.
pub fn run(features: Vec<(&'static str, Box<dyn Feature>)>) -> Result<()> {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .context("failed creating Tokio runtime")?;
    runtime.block_on(run_async(features))
}

async fn run_async(features: Vec<(&'static str, Box<dyn Feature>)>) -> Result<()> {
    let args = Args::parse()?;
    let runtime_options = RuntimeOptions::builder()
        .build()
        .map_err(anyhow::Error::msg)?;
    let runtime = Runtime::new_assume_tokio(runtime_options)?;
    let mut summary = SummaryWriter::open(args.summary_uri.as_deref())?;
    let mut failures = Vec::new();

    for feature_arg in &args.features {
        let (name, task_queue) = feature_arg.split_once(':').ok_or_else(|| {
            anyhow!("feature argument must be <name>:<task-queue>: {feature_arg}")
        })?;
        let Some((_, feature)) = features.iter().find(|(candidate, _)| *candidate == name) else {
            bail!("Rust feature not found: {name}");
        };

        eprintln!("Running feature {name}");
        let result = async {
            let proxy_auth = name == "client/http_proxy_auth";
            let use_proxy = proxy_auth || name == "client/http_proxy";
            let client = connect_client(&args, use_proxy, proxy_auth, feature.as_ref()).await?;
            run_feature(&runtime, client, name, task_queue, feature.as_ref()).await
        }
        .await;
        let (outcome, message) = match result {
            Ok(()) => ("PASSED", String::new()),
            Err(err) => {
                if let Some(reason) = skip_reason(&err) {
                    let message = reason.to_owned();
                    eprintln!("Feature {name} skipped: {message}");
                    ("SKIPPED", message)
                } else {
                    let message = format!("{err:#}");
                    eprintln!("Feature {name} failed: {message}");
                    failures.push(name.to_owned());
                    ("FAILED", message)
                }
            }
        };
        summary.write(&SummaryEntry {
            name,
            outcome,
            message: &message,
        })?;
    }

    if failures.is_empty() {
        eprintln!("All features passed");
        Ok(())
    } else {
        bail!(
            "{} feature(s) failed: {}",
            failures.len(),
            failures.join(", ")
        )
    }
}

async fn run_feature(
    runtime: &Runtime,
    client: Client,
    name: &str,
    task_queue: &str,
    feature: &dyn Feature,
) -> Result<()> {
    if let Some(capability) = feature.required_server_capability() {
        check_server_capability(&client, capability).await?;
    }
    let worker_options = feature.worker_options(WorkerOptions::new(task_queue).build())?;
    if !feature.uses_worker() {
        return feature
            .execute(FeatureContext {
                client,
                task_queue: task_queue.to_owned(),
                workflow_id: format!("{name}-{}", Uuid::new_v4()),
            })
            .await;
    }
    let mut worker = Worker::new(runtime, client.clone(), worker_options)
        .map_err(|err| anyhow!(err.to_string()))?;
    let shutdown = worker.shutdown_handle();
    let mut worker_run = Box::pin(worker.run());
    let feature_run = feature.execute(FeatureContext {
        client,
        task_queue: task_queue.to_owned(),
        workflow_id: format!("{name}-{}", Uuid::new_v4()),
    });

    let feature_result = tokio::select! {
        worker_result = &mut worker_run => {
            worker_result.context("worker failed")?;
            bail!("worker stopped before the feature completed")
        }
        feature_result = feature_run => feature_result,
    };
    shutdown();
    let worker_result = match tokio::time::timeout(Duration::from_secs(10), worker_run).await {
        Ok(result) => result.context("worker failed during shutdown"),
        Err(_) => Err(anyhow!(
            "worker for {name} did not stop within 10 seconds of shutdown"
        )),
    };
    feature_result?;
    worker_result
}

#[derive(Debug)]
struct FeatureSkipped {
    reason: String,
}

impl Display for FeatureSkipped {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.reason)
    }
}

impl Error for FeatureSkipped {}

fn skip(reason: impl Into<String>) -> anyhow::Error {
    FeatureSkipped {
        reason: reason.into(),
    }
    .into()
}

fn skip_reason(error: &anyhow::Error) -> Option<&str> {
    error
        .downcast_ref::<FeatureSkipped>()
        .map(|skipped| skipped.reason.as_str())
}

async fn check_server_capability(client: &Client, capability: ServerCapability) -> Result<()> {
    match capability {
        ServerCapability::Update => {
            check_update_support(
                client,
                UpdateWorkflowExecutionLifecycleStage::Completed,
                "server support for update is disabled; set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable",
            )
            .await
        }
        ServerCapability::AsyncAcceptedUpdate => {
            check_update_support(
                client,
                UpdateWorkflowExecutionLifecycleStage::Accepted,
                "server support for asynchronous (accepted) updates is disabled; set frontend.enableUpdateWorkflowExecutionAsyncAccepted=true in dynamic config to enable",
            )
            .await
        }
        ServerCapability::Deployment => check_deployment_support(client).await,
    }
}

async fn check_update_support(
    client: &Client,
    lifecycle_stage: UpdateWorkflowExecutionLifecycleStage,
    disabled_message: &str,
) -> Result<()> {
    let mut rpc_client = client.clone();
    let response = WorkflowService::update_workflow_execution(
        &mut rpc_client,
        Request::new(UpdateWorkflowExecutionRequest {
            namespace: client.options().namespace.clone(),
            workflow_execution: Some(WorkflowExecution {
                workflow_id: "__does_not_exist".to_owned(),
                run_id: String::new(),
            }),
            wait_policy: Some(WaitPolicy {
                lifecycle_stage: lifecycle_stage as i32,
            }),
            request: Some(UpdateRequest {
                meta: Some(Meta {
                    update_id: Uuid::new_v4().to_string(),
                    identity: "rust-features".to_owned(),
                }),
                input: Some(Input {
                    name: "__does_not_exist".to_owned(),
                    ..Default::default()
                }),
                ..Default::default()
            }),
            ..Default::default()
        }),
    )
    .await;

    match response {
        Ok(_) => Ok(()),
        Err(status) if status.code() == Code::NotFound => Ok(()),
        Err(status) if status.code() == Code::PermissionDenied => Err(skip(disabled_message)),
        Err(status) if status.code() == Code::Unimplemented => {
            Err(skip("server version too old to support update"))
        }
        Err(status) => Err(status.into()),
    }
}

async fn check_deployment_support(client: &Client) -> Result<()> {
    let mut rpc_client = client.clone();
    let response = WorkflowService::list_worker_deployments(
        &mut rpc_client,
        Request::new(ListWorkerDeploymentsRequest {
            namespace: client.options().namespace.clone(),
            ..Default::default()
        }),
    )
    .await;
    match response {
        Ok(_) => Ok(()),
        Err(_) => Err(skip("server does not support deployment versioning")),
    }
}

async fn connect_client(
    args: &Args,
    use_proxy: bool,
    proxy_auth: bool,
    feature: &dyn Feature,
) -> Result<Client> {
    let use_tls = args.client_cert_path.is_some()
        || args.client_key_path.is_some()
        || args.ca_cert_path.is_some()
        || args.tls_server_name.is_some();
    let target = if args.server.contains("://") {
        args.server.clone()
    } else if use_tls {
        format!("https://{}", args.server)
    } else {
        format!("http://{}", args.server)
    };
    let mut connection_options = ConnectionOptions::new(
        Url::parse(&target).with_context(|| format!("invalid server address {target}"))?,
    )
    .build();

    if use_tls {
        let client_tls_options = match (&args.client_cert_path, &args.client_key_path) {
            (Some(cert), Some(key)) => Some(
                ClientTlsOptions::builder()
                    .client_cert(
                        fs::read(cert)
                            .with_context(|| format!("failed reading client certificate {cert}"))?,
                    )
                    .client_private_key(
                        fs::read(key)
                            .with_context(|| format!("failed reading client key {key}"))?,
                    )
                    .build(),
            ),
            (Some(_), None) | (None, Some(_)) => {
                bail!("client certificate and key must be provided together")
            }
            (None, None) => None,
        };
        connection_options.tls_options = Some(
            TlsOptions::builder()
                .maybe_server_root_ca_cert(
                    args.ca_cert_path
                        .as_ref()
                        .map(fs::read)
                        .transpose()
                        .context("failed reading server CA certificate")?,
                )
                .maybe_domain(args.tls_server_name.clone())
                .maybe_client_tls_options(client_tls_options)
                .build(),
        );
    }

    if use_proxy && let Some(proxy_url) = &args.http_proxy_url {
        connection_options.http_connect_proxy = Some(
            HttpConnectProxyOptions::new(
                proxy_url
                    .strip_prefix("http://")
                    .unwrap_or(proxy_url)
                    .to_owned(),
            )
            .maybe_basic_auth(
                proxy_auth.then(|| ("proxy-user".to_owned(), "proxy-pass".to_owned())),
            )
            .build(),
        );
        connection_options.dns_load_balancing = None;
    }

    let connection = Connection::connect(connection_options).await?;
    let client_options =
        feature.client_options(ClientOptions::new(args.namespace.clone()).build())?;
    Client::new(connection, client_options).map_err(Into::into)
}

struct Args {
    server: String,
    namespace: String,
    client_cert_path: Option<String>,
    client_key_path: Option<String>,
    ca_cert_path: Option<String>,
    tls_server_name: Option<String>,
    http_proxy_url: Option<String>,
    summary_uri: Option<String>,
    features: Vec<String>,
}

impl Args {
    fn parse() -> Result<Self> {
        let mut server = None;
        let mut namespace = None;
        let mut client_cert_path = None;
        let mut client_key_path = None;
        let mut ca_cert_path = None;
        let mut tls_server_name = None;
        let mut http_proxy_url = None;
        let mut summary_uri = None;
        let mut features = Vec::new();
        let mut args = env::args().skip(1);

        while let Some(arg) = args.next() {
            let value = match arg.as_str() {
                "--server" | "--namespace" | "--client-cert-path" | "--client-key-path"
                | "--ca-cert-path" | "--tls-server-name" | "--http-proxy-url" | "--summary-uri" => {
                    Some(
                        args.next()
                            .ok_or_else(|| anyhow!("missing value for {arg}"))?,
                    )
                }
                _ if arg.starts_with('-') => bail!("unrecognized argument: {arg}"),
                _ => {
                    features.push(arg.clone());
                    None
                }
            };
            if let Some(value) = value {
                match arg.as_str() {
                    "--server" => server = Some(value),
                    "--namespace" => namespace = Some(value),
                    "--client-cert-path" => client_cert_path = Some(value),
                    "--client-key-path" => client_key_path = Some(value),
                    "--ca-cert-path" => ca_cert_path = Some(value),
                    "--tls-server-name" => tls_server_name = Some(value),
                    "--http-proxy-url" => http_proxy_url = Some(value),
                    "--summary-uri" => summary_uri = Some(value),
                    _ => unreachable!(),
                }
            }
        }

        if features.is_empty() {
            bail!("at least one feature is required")
        }
        Ok(Self {
            server: server.context("--server is required")?,
            namespace: namespace.context("--namespace is required")?,
            client_cert_path,
            client_key_path,
            ca_cert_path,
            tls_server_name,
            http_proxy_url,
            summary_uri,
            features,
        })
    }
}

#[derive(Serialize)]
struct SummaryEntry<'a> {
    name: &'a str,
    outcome: &'a str,
    message: &'a str,
}

enum SummaryWriter {
    Tcp(BufWriter<TcpStream>),
    File(BufWriter<File>),
    None,
}

impl SummaryWriter {
    fn open(uri: Option<&str>) -> Result<Self> {
        let Some(uri) = uri else {
            return Ok(Self::None);
        };
        if let Some(address) = uri.strip_prefix("tcp://") {
            Ok(Self::Tcp(BufWriter::new(
                TcpStream::connect(address)
                    .with_context(|| format!("failed connecting to summary address {address}"))?,
            )))
        } else if let Some(path) = uri.strip_prefix("file://") {
            Ok(Self::File(BufWriter::new(
                File::create(path)
                    .with_context(|| format!("failed creating summary file {path}"))?,
            )))
        } else {
            bail!("unsupported summary URI: {uri}")
        }
    }

    fn write(&mut self, entry: &SummaryEntry<'_>) -> Result<()> {
        let writer: &mut dyn Write = match self {
            Self::Tcp(writer) => writer,
            Self::File(writer) => writer,
            Self::None => return Ok(()),
        };
        serde_json::to_writer(&mut *writer, entry)?;
        writer.write_all(b"\n")?;
        writer.flush()?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn skip_errors_preserve_their_reason() {
        let error = skip("not supported");
        assert_eq!(skip_reason(&error), Some("not supported"));
    }
}
