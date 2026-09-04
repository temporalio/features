use anyhow::{Result, ensure};
use std::time::Duration;
use temporalio_client::WorkflowGetResultOptions;
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{
    ChildWorkflowOptions, SignalWorkflowOptions, SyncWorkflowContext, WorkerOptions, WorkflowContext, WorkflowResult,
};

const UNBLOCK_MESSAGE: &str = "unblock";

#[workflow]
#[derive(Default)]
pub struct SignalChildWorkflow {
    message: Option<String>,
}

#[workflow_methods]
impl SignalChildWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        ctx.wait_condition(|state| state.message.is_some()).await?;
        Ok(ctx.state(|state| state.message.clone().unwrap_or_default()))
    }

    #[signal(name = "unblock-signal")]
    pub fn unblock(&mut self, _ctx: &mut SyncWorkflowContext<Self>, message: String) {
        self.message = Some(message);
    }
}

#[workflow]
#[derive(Default)]
pub struct SignalWorkflow;

#[workflow_methods]
impl SignalWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        let child = ctx
            .start_child_workflow(
                SignalChildWorkflow::run,
                (),
                ChildWorkflowOptions::builder()
                    .workflow_id("signal-child".to_owned())
                    .execution_timeout(Duration::from_secs(600))
                    .task_timeout(Duration::from_secs(60))
                    .build(),
            )
            .await?;
        child
            .signal(
                SignalChildWorkflow::unblock,
                UNBLOCK_MESSAGE.to_owned(),
                SignalWorkflowOptions::default(),
            )
            .await?;
        Ok(child.result().await?)
    }
}

struct SignalChildFeature;

#[async_trait]
impl Feature for SignalChildFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options
            .register_workflow::<SignalWorkflow>()?
            .register_workflow::<SignalChildWorkflow>()?;
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let handle = context
            .client
            .start_workflow(
                SignalWorkflow::run,
                (),
                context.workflow_start_options(),
            )
            .await?;
        let result = handle
            .get_result(WorkflowGetResultOptions::default())
            .await?;
        ensure!(result == UNBLOCK_MESSAGE, "unexpected signal result: {result}");
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    SignalChildFeature
}
