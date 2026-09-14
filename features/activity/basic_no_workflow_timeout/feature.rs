use anyhow::{Result, ensure};
use std::time::Duration;
use temporalio_client::WorkflowGetResultOptions;
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{activities, workflow, workflow_methods};
use temporalio_sdk::{
    ActivityOptions, WorkerOptions, WorkflowContext, WorkflowResult,
    activities::{ActivityContext, ActivityError},
};

#[workflow]
#[derive(Default)]
pub struct BasicActivityWorkflow;

#[workflow_methods]
impl BasicActivityWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        #[allow(deprecated)]
        ctx.start_activity(
            BasicActivityActivities::echo,
            (),
            ActivityOptions::with_start_to_close_timeout(Duration::from_secs(60)).build(),
        )
        .await?;

        #[allow(deprecated)]
        let result = ctx
            .start_activity(
                BasicActivityActivities::echo,
                (),
                ActivityOptions::with_schedule_to_close_timeout(Duration::from_secs(60)).build(),
            )
            .await?;
        Ok(result)
    }
}

pub struct BasicActivityActivities;

#[activities]
impl BasicActivityActivities {
    #[activity]
    pub async fn echo(_ctx: ActivityContext) -> Result<String, ActivityError> {
        Ok("echo".to_owned())
    }
}

struct BasicActivityFeature;

#[async_trait]
impl Feature for BasicActivityFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options
            .register_workflow::<BasicActivityWorkflow>()?
            .register_activities(BasicActivityActivities);
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let mut start_options = context.workflow_start_options();
        start_options.execution_timeout = None;
        let handle = context
            .client
            .start_workflow(BasicActivityWorkflow::run, (), start_options)
            .await?;
        let result = handle
            .get_result(WorkflowGetResultOptions::default())
            .await?;
        ensure!(result == "echo", "unexpected workflow result: {result}");
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    BasicActivityFeature
}
