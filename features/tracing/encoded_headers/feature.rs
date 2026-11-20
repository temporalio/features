use crate::harness::{Feature, FeatureContext, FeatureFuture, FeatureOutcome};
use anyhow::{Result, ensure};
use futures::{FutureExt, future::BoxFuture};
use opentelemetry::{
    Context,
    trace::{FutureExt as _, TraceContextExt, Tracer, TracerProvider},
};
use prost::Message;
use std::{collections::HashMap, time::Duration};
use temporalio_client::{Client, ClientOptions, WorkflowGetResultOptions};
use temporalio_common::{
    data_converters::{
        DataConverter, DefaultFailureConverter, PayloadCodec, PayloadConversionError,
        PayloadConverter, SerializationContextData,
    },
    protos::temporal::api::{common::v1::Payload, history::v1::history_event::Attributes},
};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{
    WorkerOptions, WorkflowContext, WorkflowResult, opentelemetry::OpenTelemetryPlugin,
};

const ENCODING: &[u8] = b"binary/encrypted";

#[derive(Debug)]
struct WrappingCodec;

impl PayloadCodec for WrappingCodec {
    fn encode(
        &self,
        _: &SerializationContextData,
        payloads: Vec<Payload>,
    ) -> BoxFuture<'static, Result<Vec<Payload>, PayloadConversionError>> {
        async move {
            payloads
                .into_iter()
                .map(|payload| {
                    if payload.metadata.get("encoding").map(Vec::as_slice) == Some(ENCODING) {
                        return Err(PayloadConversionError::EncodingError(
                            "Payload encoded twice".into(),
                        ));
                    }
                    Ok(Payload {
                        metadata: HashMap::from([("encoding".to_owned(), ENCODING.to_vec())]),
                        data: payload
                            .encode_to_vec()
                            .into_iter()
                            .map(|b| b ^ 0x5a)
                            .collect(),
                        ..Default::default()
                    })
                })
                .collect()
        }
        .boxed()
    }

    fn decode(
        &self,
        _: &SerializationContextData,
        payloads: Vec<Payload>,
    ) -> BoxFuture<'static, Result<Vec<Payload>, PayloadConversionError>> {
        async move {
            payloads
                .into_iter()
                .map(|payload| {
                    if payload.metadata.get("encoding").map(Vec::as_slice) != Some(ENCODING) {
                        return Err(PayloadConversionError::EncodingError(
                            "Expected encoded payload".into(),
                        ));
                    }
                    let data: Vec<u8> = payload.data.into_iter().map(|b| b ^ 0x5a).collect();
                    Payload::decode(data.as_slice())
                        .map_err(|e| PayloadConversionError::EncodingError(e.into()))
                })
                .collect()
        }
        .boxed()
    }
}

#[workflow]
#[derive(Default)]
pub struct Workflow;

#[workflow_methods]
impl Workflow {
    #[run(name = "workflow")]
    pub async fn run(
        _ctx: &mut WorkflowContext<Self>,
        message: String,
    ) -> WorkflowResult<HashMap<String, String>> {
        let context = Context::current();
        let span = context.span();
        assert!(
            span.span_context().is_valid(),
            "No active workflow tracing span"
        );
        Ok(HashMap::from([
            ("message".to_owned(), message),
            (
                "traceId".to_owned(),
                span.span_context().trace_id().to_string(),
            ),
        ]))
    }
}

struct EncodedHeaders {
    provider: opentelemetry_sdk::trace::SdkTracerProvider,
}

impl Feature for EncodedHeaders {
    fn configure_client(&self, client: Client) -> Result<Client> {
        let options = ClientOptions::new(client.options().namespace.clone())
            .data_converter(DataConverter::new(
                PayloadConverter::default(),
                DefaultFailureConverter::default(),
                WrappingCodec,
            ))
            .plugin(
                OpenTelemetryPlugin::builder()
                    .tracer(self.provider.tracer("encoded-headers-feature"))
                    .encode_client_headers(true)
                    .build(),
            )
            .build();
        Ok(Client::new(client.connection().clone(), options)?)
    }

    fn worker_options(&self, ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(Some(
            WorkerOptions::new(ctx.task_queue.clone())
                .register_workflow::<Workflow>()?
                .build(),
        ))
    }

    fn execute<'a>(&'a self, ctx: &'a FeatureContext) -> FeatureFuture<'a> {
        Box::pin(async move {
            let tracer = self.provider.tracer("encoded-headers-feature");
            let parent = Context::new().with_span(tracer.start("rust-caller"));
            let trace_id = parent.span().span_context().trace_id().to_string();
            let result = async {
                let mut options = ctx.start_options();
                options.execution_timeout = Some(Duration::from_secs(15));
                let handle = ctx
                    .client
                    .start_workflow(Workflow::run, "hello".to_owned(), options)
                    .await?;
                let result = handle.get_result(WorkflowGetResultOptions::default()).await;
                let events = handle
                    .fetch_history(Default::default())
                    .into_events()
                    .await?;
                for event in &events {
                    ensure!(
                        !matches!(
                            event.attributes,
                            Some(Attributes::WorkflowTaskFailedEventAttributes(_))
                        ),
                        "Workflow Task failed: {event:?}"
                    );
                }
                let Some(Attributes::WorkflowExecutionStartedEventAttributes(started)) =
                    events.first().and_then(|event| event.attributes.as_ref())
                else {
                    anyhow::bail!("Missing workflow start event");
                };
                let header = &started.header.as_ref().unwrap().fields["_tracer-data"];
                let decoded = WrappingCodec
                    .decode(
                        &SerializationContextData::Workflow(
                            temporalio_common::data_converters::WorkflowSerializationContext::new(),
                        ),
                        vec![header.clone()],
                    )
                    .await?;
                let carrier: HashMap<String, String> = serde_json::from_slice(&decoded[0].data)?;
                ensure!(
                    carrier["traceparent"].split('-').nth(1) == Some(trace_id.as_str()),
                    "History trace identity mismatch"
                );
                ensure!(
                    result?
                        == HashMap::from([
                            ("message".to_owned(), "hello".to_owned()),
                            ("traceId".to_owned(), trace_id),
                        ]),
                    "Workflow input/result or trace identity mismatch"
                );
                Ok(FeatureOutcome::Passed)
            }
            .with_context(parent.clone())
            .await;
            parent.span().end();
            result
        })
    }
}

pub fn feature() -> Box<dyn Feature> {
    let provider = opentelemetry_sdk::trace::SdkTracerProvider::builder()
        .with_id_generator(temporalio_sdk::opentelemetry::WorkflowIdGenerator::default())
        .build();
    Box::new(EncodedHeaders { provider })
}
