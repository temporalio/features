use anyhow::{Result, ensure};
use std::time::Duration;
use temporalio_client::WorkflowGetResultOptions;
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{
    ChildWorkflowOptions, WorkerOptions, WorkflowContext, WorkflowResult,
};

const CHILD_WORKFLOW_INPUT: &str = "test";

#[workflow]
#[derive(Default)]
pub struct ResultChildWorkflow;

#[workflow_methods]
impl ResultChildWorkflow {
    #[run]
    pub async fn run(
        _ctx: &mut WorkflowContext<Self>,
        input: String,
    ) -> WorkflowResult<String> {
        Ok(input)
    }
}

#[workflow]
#[derive(Default)]
pub struct ResultWorkflow;

#[workflow_methods]
impl ResultWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        let child = ctx
            .start_child_workflow(
                ResultChildWorkflow::run,
                CHILD_WORKFLOW_INPUT.to_owned(),
                ChildWorkflowOptions::builder()
                    .workflow_id("result-child".to_owned())
                    .execution_timeout(Duration::from_secs(600))
                    .task_timeout(Duration::from_secs(60))
                    .build(),
            )
            .await?;
        Ok(child.result().await?)
    }
}

struct ResultFeature;

#[async_trait]
impl Feature for ResultFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options
            .register_workflow::<ResultWorkflow>()?
            .register_workflow::<ResultChildWorkflow>()?;
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let handle = context
            .client
            .start_workflow(
                ResultWorkflow::run,
                (),
                context.workflow_start_options(),
            )
            .await?;
        let result = handle
            .get_result(WorkflowGetResultOptions::default())
            .await?;
        ensure!(
            result == CHILD_WORKFLOW_INPUT,
            "unexpected child workflow result: {result}"
        );
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    ResultFeature
}
