use anyhow::{Result, ensure};
use temporalio_client::{
    WorkflowGetResultOptions, WorkflowSignalOptions,
};
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{SyncWorkflowContext, WorkerOptions, WorkflowContext, WorkflowResult};

const SIGNAL_DATA: &str = "signal-data";

#[workflow]
#[derive(Default)]
pub struct BasicSignalWorkflow {
    result: Option<String>,
}

#[workflow_methods]
impl BasicSignalWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        ctx.wait_condition(|state| state.result.is_some()).await?;
        Ok(ctx.state(|state| state.result.clone().unwrap_or_default()))
    }

    #[signal(name = "mySignal")]
    pub fn receive(&mut self, _ctx: &mut SyncWorkflowContext<Self>, value: String) {
        self.result = Some(value);
    }
}

struct BasicSignalFeature;

#[async_trait]
impl Feature for BasicSignalFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options.register_workflow::<BasicSignalWorkflow>()?;
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let handle = context
            .client
            .start_workflow(
                BasicSignalWorkflow::run,
                (),
                context.workflow_start_options(),
            )
            .await?;
        handle
            .signal(
                BasicSignalWorkflow::receive,
                SIGNAL_DATA.to_owned(),
                WorkflowSignalOptions::default(),
            )
            .await?;
        let result = handle
            .get_result(WorkflowGetResultOptions::default())
            .await?;
        ensure!(result == SIGNAL_DATA, "unexpected signal result: {result}");
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    BasicSignalFeature
}
