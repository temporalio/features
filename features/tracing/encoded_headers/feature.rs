use crate::harness::{Feature, FeatureContext, FeatureFuture};
use anyhow::{Result, bail};
use futures::{FutureExt, future::BoxFuture};
use opentelemetry::{
    Context,
    trace::{TraceContextExt, TracerProvider},
};
use prost::Message;
use std::collections::HashMap;
use temporalio_client::{Client, ClientOptions};
use temporalio_common::{
    data_converters::{
        DataConverter, DefaultFailureConverter, PayloadCodec, PayloadConversionError,
        PayloadConverter, SerializationContextData,
    },
    protos::temporal::api::common::v1::Payload,
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
            .build();
        Ok(Client::new(client.connection().clone(), options)?)
    }

    fn worker_options(&self, ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(Some(
            WorkerOptions::new(ctx.task_queue.clone())
                .worker_plugin(
                    OpenTelemetryPlugin::builder()
                        .tracer(self.provider.tracer("encoded-headers-feature"))
                        .build(),
                )
                .register_workflow::<Workflow>()?
                .build(),
        ))
    }

    fn execute<'a>(&'a self, _: &'a FeatureContext) -> FeatureFuture<'a> {
        Box::pin(async {
            bail!("Rust is worker-only: the client tracing producer has no codec opt-in")
        })
    }
}

pub fn feature() -> Box<dyn Feature> {
    let provider = opentelemetry_sdk::trace::SdkTracerProvider::builder()
        .with_id_generator(temporalio_sdk::opentelemetry::WorkflowIdGenerator::default())
        .build();
    Box::new(EncodedHeaders { provider })
}
