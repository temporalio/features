use anyhow::{Result, ensure};
use temporalio_client::{
    WorkflowGetResultOptions, WorkflowQueryOptions, WorkflowSignalOptions,
};
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{
    SyncWorkflowContext, WorkerOptions, WorkflowContext, WorkflowContextView, WorkflowResult,
};

#[workflow]
#[derive(Default)]
pub struct SuccessfulQueryWorkflow {
    counter: i32,
}

#[workflow_methods]
impl SuccessfulQueryWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<()> {
        ctx.wait_condition(|state| state.counter == 5).await?;
        Ok(())
    }

    #[signal(name = "counterInc")]
    pub fn increment(&mut self, _ctx: &mut SyncWorkflowContext<Self>) {
        self.counter += 1;
    }

    #[query(name = "counterQ")]
    pub fn counter(&self, _ctx: &WorkflowContextView) -> i32 {
        self.counter
    }
}

struct SuccessfulQueryFeature;

#[async_trait]
impl Feature for SuccessfulQueryFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options.register_workflow::<SuccessfulQueryWorkflow>()?;
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let handle = context
            .client
            .start_workflow(
                SuccessfulQueryWorkflow::run,
                (),
                context.workflow_start_options(),
            )
            .await?;

        let result = handle
            .query(
                SuccessfulQueryWorkflow::counter,
                (),
                WorkflowQueryOptions::default(),
            )
            .await?;
        ensure!(result == 0, "expected counter 0, got {result}");

        handle
            .signal(
                SuccessfulQueryWorkflow::increment,
                (),
                WorkflowSignalOptions::default(),
            )
            .await?;
        let result = handle
            .query(
                SuccessfulQueryWorkflow::counter,
                (),
                WorkflowQueryOptions::default(),
            )
            .await?;
        ensure!(result == 1, "expected counter 1, got {result}");

        for _ in 0..3 {
            handle
                .signal(
                    SuccessfulQueryWorkflow::increment,
                    (),
                    WorkflowSignalOptions::default(),
                )
                .await?;
        }
        let result = handle
            .query(
                SuccessfulQueryWorkflow::counter,
                (),
                WorkflowQueryOptions::default(),
            )
            .await?;
        ensure!(result == 4, "expected counter 4, got {result}");

        handle
            .signal(
                SuccessfulQueryWorkflow::increment,
                (),
                WorkflowSignalOptions::default(),
            )
            .await?;
        handle
            .get_result(WorkflowGetResultOptions::default())
            .await?;
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    SuccessfulQueryFeature
}
