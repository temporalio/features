//! Feature authoring and supervised Temporal execution.

use crate::{
    options::Options,
    summary::{Entry, Summary},
};
use anyhow::{Context, Result, bail, ensure};
use futures::{FutureExt, TryStreamExt};
use serde_json::{Map, Value};
use std::{
    collections::BTreeMap, future::Future, panic::AssertUnwindSafe, pin::Pin, time::Duration,
};
use temporalio_client::{
    Client, ClientOptions, Connection, WorkflowExecutionInfo, WorkflowExecutionStatus,
    WorkflowHistory, WorkflowListOptions, WorkflowStartOptions,
};
use temporalio_common::protos::temporal::api::{
    common::v1::WorkflowExecution,
    history::v1::{History, history_event::Attributes},
};
use temporalio_sdk::{
    Runtime, Worker, WorkerOptions,
    workflow_replayer::{WorkflowReplayer, WorkflowReplayerOptions},
};
use tokio_util::sync::CancellationToken;

/// A local future: SDK workers and feature drivers need not be `Send`.
pub type FeatureFuture<'a> = Pin<Box<dyn Future<Output = Result<FeatureOutcome>> + 'a>>;

/// A successful feature can explicitly report that it does not apply.
pub enum FeatureOutcome {
    Passed,
    Skipped(String),
}

/// The compile-time registry contains one factory result per feature path.
pub type Registry = Vec<(&'static str, Box<dyn Feature>)>;

/// Each `feature.rs` exports `pub fn feature() -> Box<dyn Feature>`.
///
/// Registration remains typed inside the module. Return worker options to have
/// the harness own polling, cancellation, and shutdown. Client-only features can
/// leave worker options unset. Return replay options for workflow features;
/// the harness discovers their current histories after worker shutdown.
pub trait Feature {
    /// Override data conversion, interceptors, or other public client options.
    fn configure_client(&self, client: Client) -> Result<Client> {
        Ok(client)
    }

    /// Build the worker using the assigned task queue, or return no worker.
    fn worker_options(&self, _ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(None)
    }

    /// Register the same workflow implementations used by the live worker.
    ///
    /// `None` explicitly disables native replay for client/activity-only features
    /// or features with a documented reason they cannot replay.
    fn replay_options(&self, _ctx: &FeatureContext) -> Result<Option<WorkflowReplayerOptions>> {
        Ok(None)
    }

    /// Start executions, send messages, and make feature-specific assertions.
    fn execute<'a>(&'a self, ctx: &'a FeatureContext) -> FeatureFuture<'a>;
}

/// Per-selection context with isolated workflow IDs and task queues.
pub struct FeatureContext {
    pub client: Client,
    pub name: String,
    pub namespace: String,
    pub task_queue: String,
    pub nexus_endpoint: Option<String>,
    /// The complete object from `FEATURE_NAMESPACE_CAPABILITIES`.
    pub namespace_capabilities: Map<String, Value>,
}

impl FeatureContext {
    /// Start one workflow per feature using its isolated queue as execution ID.
    /// The execution timeout defaults to one minute; features may override it.
    pub fn start_options(&self) -> WorkflowStartOptions {
        WorkflowStartOptions::new(self.task_queue.clone(), self.task_queue.clone())
            .execution_timeout(Duration::from_secs(60))
            .build()
    }

    /// Register a single workflow with the same namespace and queue as execution.
    pub fn replay_options<W: temporalio_sdk::workflows::WorkflowImplementation>(
        &self,
    ) -> Result<WorkflowReplayerOptions>
    where
        <W::Run as temporalio_common::WorkflowDefinition>::Input: Send,
    {
        Ok(WorkflowReplayerOptions::new()
            .namespace(self.namespace.clone())
            .task_queue(self.task_queue.clone())
            .register_workflow::<W>()?
            .build())
    }

    /// Missing capabilities are not assumed supported.
    pub fn has_capability(&self, name: &str) -> bool {
        self.namespace_capabilities
            .get(name)
            .and_then(Value::as_bool)
            == Some(true)
    }
}

const SHUTDOWN_TIMEOUT: Duration = Duration::from_secs(30);
const FEATURE_TIMEOUT: Duration = Duration::from_secs(180);

/// Execute all selections and close the summary transport before returning.
///
/// `Ok(false)` means one or more failed selections; configuration/transport
/// failures return `Err`. Unknown feature names are reported as failed entries.
pub async fn run(args: impl Iterator<Item = String>, registry: Registry) -> Result<bool> {
    let options = Options::parse(args)?;
    let capabilities_json = match std::env::var("FEATURE_NAMESPACE_CAPABILITIES") {
        Ok(value) => Some(value),
        Err(std::env::VarError::NotPresent) => None,
        Err(error) => return Err(error).context("read FEATURE_NAMESPACE_CAPABILITIES"),
    };
    let capabilities = parse_capabilities(capabilities_json.as_deref())?;
    let connection_options = options.connection_options()?;
    let mut summary = Summary::open(options.summary_uri.as_deref())?;
    let cancelled = CancellationToken::new();
    let signal_task = tokio::spawn({
        let cancelled = cancelled.clone();
        async move {
            if let Err(error) = wait_for_signal().await {
                eprintln!("signal handler failed: {error:#}");
            }
            cancelled.cancel();
        }
    });
    // Dropping the handle must not leave a signal task alive on an early error.
    struct SignalGuard(tokio::task::JoinHandle<()>);
    impl Drop for SignalGuard {
        fn drop(&mut self) {
            self.0.abort();
        }
    }
    let _signal_guard = SignalGuard(signal_task);
    let runtime = Runtime::from_current_tokio(Default::default())?;
    let mut base_client = None;
    let mut all_passed = true;
    for selection in &options.selections {
        eprintln!("Running Rust feature {}", selection.name);
        let result = async {
            let feature = registry
                .iter()
                .find(|(name, _)| *name == selection.name)
                .map(|(_, feature)| feature)
                .with_context(|| format!("unknown Rust feature {}", selection.name))?;
            ensure!(!cancelled.is_cancelled(), "execution cancelled");
            if base_client.is_none() {
                let connection = tokio::select! {
                    result = tokio::time::timeout(Duration::from_secs(20),
                        Connection::connect(connection_options.clone())) => result.context("connection timed out")??,
                    _ = cancelled.cancelled() => bail!("execution cancelled"),
                };
                base_client = Some(Client::new(
                    connection,
                    ClientOptions::new(options.namespace.clone()).build(),
                )?);
            }
            let ctx = FeatureContext {
                client: feature.configure_client(base_client.as_ref().unwrap().clone())?,
                name: selection.name.clone(),
                namespace: options.namespace.clone(),
                task_queue: selection.task_queue.clone(),
                nexus_endpoint: selection.nexus_endpoint.clone(),
                namespace_capabilities: capabilities.clone(),
            };
            run_feature(&runtime, &**feature, &ctx, &options, &cancelled).await
        };
        let result = catch(result).await;
        let (outcome, message) = match result {
            Ok(FeatureOutcome::Passed) => ("PASSED", String::new()),
            Ok(FeatureOutcome::Skipped(message)) => ("SKIPPED", message),
            Err(error) => {
                all_passed = false;
                ("FAILED", format!("{error:#}"))
            }
        };
        eprintln!("Rust feature {}: {outcome} {message}", selection.name);
        summary.write(&Entry {
            name: &selection.name,
            outcome,
            message: &message,
        })?;
    }
    // No background writer owns the socket: dropping it delivers EOF promptly.
    drop(summary);
    Ok(all_passed)
}

async fn run_feature(
    runtime: &Runtime,
    feature: &dyn Feature,
    ctx: &FeatureContext,
    options: &Options,
    cancelled: &CancellationToken,
) -> Result<FeatureOutcome> {
    let worker_options = feature.worker_options(ctx)?;
    let driver = async {
        tokio::select! {
            result = tokio::time::timeout(FEATURE_TIMEOUT, catch(feature.execute(ctx))) =>
                result.context("feature execution timed out")?,
            _ = cancelled.cancelled() => bail!("execution cancelled"),
        }
    };
    let outcome = if let Some(mut worker_options) = worker_options {
        // Give outstanding activities a finite grace period before SDK cancellation.
        if worker_options.graceful_shutdown_period.is_none() {
            worker_options.graceful_shutdown_period = Some(Duration::from_secs(5));
        }
        let mut worker = Worker::new(runtime, ctx.client.clone(), worker_options)?;
        let shutdown = worker.shutdown_handle();
        supervise(worker.run(), shutdown, driver, SHUTDOWN_TIMEOUT).await?
    } else {
        driver.await?
    };
    if matches!(outcome, FeatureOutcome::Skipped(_)) || options.no_history_check {
        return Ok(outcome);
    }
    if let Some(replay_options) = feature.replay_options(ctx)? {
        let executions = tokio::select! {
            result = current_executions(ctx) => result?,
            _ = cancelled.cancelled() => bail!("execution cancelled during history discovery"),
        };
        for (index, execution) in executions.into_iter().enumerate() {
            eprintln!(
                "Replaying current Rust history {} for {}: {} (run: {})",
                index + 1,
                ctx.name,
                execution.workflow_id,
                execution.run_id,
            );
            let replay = async {
                let history = current_history(ctx, execution).await?;
                let replayer = WorkflowReplayer::new(replay_options.clone())?;
                replayer.replay_workflow(history).await?;
                Ok::<_, anyhow::Error>(())
            };
            tokio::select! {
                result = tokio::time::timeout(FEATURE_TIMEOUT, replay) => {
                    result.context("native replay timed out")?
                        .with_context(|| format!("current native replay history {}", index + 1))?;
                }
                _ = cancelled.cancelled() => bail!("execution cancelled during native replay"),
            }
        }
    }
    Ok(outcome)
}

async fn current_executions(ctx: &FeatureContext) -> Result<Vec<WorkflowExecution>> {
    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    // Visibility uses SQL string literals with backslash escapes.
    let query = format!(
        "TaskQueue = '{}'",
        ctx.task_queue.replace('\\', "\\\\").replace('\'', "''")
    );
    loop {
        let listing = async {
            let mut executions = BTreeMap::new();
            let mut listing = ctx
                .client
                .list_workflows(query.clone(), WorkflowListOptions::default());
            while let Some(info) = listing
                .try_next()
                .await
                .context("list current workflow executions")?
            {
                ensure!(
                    !info.id().is_empty() && !info.run_id().is_empty(),
                    "listed workflow has an empty workflow or run ID"
                );
                executions.insert(
                    (info.id().to_owned(), info.run_id().to_owned()),
                    info.status(),
                );
            }
            Ok::<_, anyhow::Error>(executions)
        };
        let executions = tokio::time::timeout_at(deadline, listing)
            .await
            .context("current workflow listing timed out after 5s")??;
        let running: Vec<_> = executions
            .iter()
            .filter(|(_, status)| **status == WorkflowExecutionStatus::Running)
            .map(|((id, run), _)| format!("{id} (run: {run})"))
            .collect();
        if !executions.is_empty() && running.is_empty() {
            return Ok(executions
                .into_keys()
                .map(|(workflow_id, run_id)| WorkflowExecution {
                    workflow_id,
                    run_id,
                })
                .collect());
        }
        ensure!(
            tokio::time::Instant::now() + Duration::from_millis(100) < deadline,
            "after 5s, current workflows for queue {:?}: {} found, still running: {}",
            ctx.task_queue,
            executions.len(),
            running.join(", ")
        );
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

async fn current_history(
    ctx: &FeatureContext,
    execution: WorkflowExecution,
) -> Result<WorkflowHistory> {
    let events = WorkflowExecutionInfo::builder()
        .namespace(ctx.namespace.clone())
        .workflow_id(execution.workflow_id.clone())
        .run_id(execution.run_id)
        .build()
        .bind_untyped(ctx.client.clone())
        .fetch_history(Default::default())
        .into_events()
        .await
        .context("fetch current workflow history")?;
    validated_history(History { events }, &execution.workflow_id)
}

fn validated_history(mut history: History, workflow_id: &str) -> Result<WorkflowHistory> {
    // Core assumes the first event contains a run ID and panics otherwise.
    ensure!(
        matches!(
            history.events.first().and_then(|event| event.attributes.as_ref()),
            Some(Attributes::WorkflowExecutionStartedEventAttributes(started))
                if !started.original_execution_run_id.is_empty()
        ),
        "native history must begin with WorkflowExecutionStarted containing a run ID"
    );
    // Older servers may omit the workflow ID in the event. Preserve the ID
    // supplied by visibility when rebuilding the eagerly fetched history.
    if let Some(Attributes::WorkflowExecutionStartedEventAttributes(started)) = history
        .events
        .first_mut()
        .and_then(|event| event.attributes.as_mut())
        && started.workflow_id.is_empty()
    {
        started.workflow_id = workflow_id.to_owned();
    }
    Ok(history.into())
}

/// Poll the worker and driver on the same task; never spawn the local worker.
async fn supervise<T>(
    worker: impl Future<Output = Result<(), temporalio_sdk::WorkerRunError>>,
    shutdown: impl Fn(),
    driver: impl Future<Output = Result<T>>,
    shutdown_timeout: Duration,
) -> Result<T> {
    let worker = catch(async { worker.await.map_err(Into::into) });
    tokio::pin!(worker);
    let result = tokio::select! {
        result = &mut worker => {
            shutdown();
            result.context("worker stopped before feature execution completed")?;
            bail!("worker stopped before feature execution completed");
        }
        result = catch(driver) => result,
    };
    shutdown();
    let drained = tokio::time::timeout(shutdown_timeout, &mut worker)
        .await
        .context("worker shutdown timed out after requesting activity cancellation");
    match (result, drained) {
        (Ok(value), Ok(Ok(()))) => Ok(value),
        (Err(error), Ok(Ok(()))) => Err(error),
        (Ok(_), Ok(Err(error))) => Err(error.context("worker shutdown")),
        (Ok(_), Err(error)) => Err(error),
        (Err(error), Ok(Err(shutdown))) => {
            Err(error.context(format!("worker shutdown also failed: {shutdown:#}")))
        }
        (Err(error), Err(shutdown)) => Err(error.context(format!("{shutdown:#}"))),
    }
}

async fn catch<T>(future: impl Future<Output = Result<T>>) -> Result<T> {
    AssertUnwindSafe(future)
        .catch_unwind()
        .await
        .unwrap_or_else(|panic| {
            let message = panic
                .downcast_ref::<String>()
                .map(String::as_str)
                .or_else(|| panic.downcast_ref::<&str>().copied())
                .unwrap_or("non-string panic");
            Err(anyhow::anyhow!("panic: {message}"))
        })
}

fn parse_capabilities(value: Option<&str>) -> Result<Map<String, Value>> {
    match value {
        None | Some("") => Ok(Map::new()),
        Some(value) => serde_json::from_str(value)
            .context("FEATURE_NAMESPACE_CAPABILITIES must be a JSON object"),
    }
}

async fn wait_for_signal() -> Result<()> {
    #[cfg(unix)]
    {
        let mut terminate =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
        tokio::select! {
            result = tokio::signal::ctrl_c() => result?,
            _ = terminate.recv() => {}
        }
    }
    #[cfg(not(unix))]
    tokio::signal::ctrl_c().await?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn capabilities_require_an_object() {
        assert!(parse_capabilities(None).unwrap().is_empty());
        assert_eq!(
            parse_capabilities(Some(r#"{"x":true}"#)).unwrap()["x"],
            true
        );
        for value in ["[]", "null", "{"] {
            assert!(parse_capabilities(Some(value)).is_err());
        }
    }

    #[test]
    fn malformed_native_history_is_rejected_before_replay() {
        assert!(
            validated_history(History::default(), "workflow")
                .unwrap_err()
                .to_string()
                .contains("WorkflowExecutionStarted")
        );
    }

    #[tokio::test]
    async fn driver_error_and_panic_drain_worker() {
        for panic in [false, true] {
            let stopped = CancellationToken::new();
            let drained = std::cell::Cell::new(false);
            let worker = async {
                stopped.cancelled().await;
                drained.set(true);
                Ok(())
            };
            let driver = async {
                if panic {
                    panic!("driver panic");
                }
                bail!("driver error")
            };
            let result: Result<()> =
                supervise(worker, || stopped.cancel(), driver, Duration::from_secs(1)).await;
            assert!(result.is_err());
            assert!(drained.get());
        }
    }

    #[tokio::test]
    async fn early_worker_exit_and_shutdown_timeout_fail() {
        let result: Result<()> = supervise(
            async { Ok(()) },
            || {},
            std::future::pending(),
            Duration::from_millis(1),
        )
        .await;
        assert!(result.unwrap_err().to_string().contains("worker stopped"));
        let result = supervise(
            std::future::pending(),
            || {},
            async { Ok(()) },
            Duration::from_millis(1),
        )
        .await;
        assert!(
            result
                .unwrap_err()
                .to_string()
                .contains("shutdown timed out")
        );
    }
}
