use crate::harness::{Feature, FeatureContext, FeatureFuture, FeatureOutcome};
use anyhow::{Result, ensure};
use std::time::Duration;
use temporalio_client::WorkflowGetResultOptions;
use temporalio_macros::{activities, workflow, workflow_methods};
use temporalio_sdk::{
    ActivityOptions, WorkerOptions, WorkflowContext, WorkflowResult,
    activities::{ActivityContext, ActivityError},
    workflow_replayer::WorkflowReplayerOptions,
};

#[workflow]
#[derive(Default)]
pub struct Workflow;

#[workflow_methods]
impl Workflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        let first = ctx
            .execute_activity(
                Activities::echo,
                (),
                ActivityOptions::start_to_close_timeout(Duration::from_secs(60)),
            )
            .await?;
        assert_eq!(first, "echo");
        let second = ctx
            .execute_activity(
                Activities::echo,
                (),
                ActivityOptions::schedule_to_close_timeout(Duration::from_secs(60)),
            )
            .await?;
        Ok(second)
    }
}

pub struct Activities;

#[activities]
impl Activities {
    #[activity]
    pub async fn echo(_ctx: ActivityContext) -> Result<String, ActivityError> {
        Ok("echo".to_owned())
    }
}

struct BasicActivity;

impl Feature for BasicActivity {
    fn worker_options(&self, ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(Some(
            WorkerOptions::new(ctx.task_queue.clone())
                .register_workflow::<Workflow>()?
                .register_activities(Activities)
                .build(),
        ))
    }

    fn replay_options(&self, ctx: &FeatureContext) -> Result<Option<WorkflowReplayerOptions>> {
        Ok(Some(ctx.replay_options::<Workflow>()?))
    }

    fn execute<'a>(&'a self, ctx: &'a FeatureContext) -> FeatureFuture<'a> {
        Box::pin(async move {
            // Leave both workflow execution and run timeouts unset.
            let mut options = ctx.start_options();
            options.execution_timeout = None;
            let handle = ctx
                .client
                .start_workflow(Workflow::run, (), options)
                .await?;
            ensure!(
                handle.get_result(WorkflowGetResultOptions::default()).await? == "echo",
                "unexpected activity result"
            );
            Ok(FeatureOutcome::Passed)
        })
    }
}

pub fn feature() -> Box<dyn Feature> {
    Box::new(BasicActivity)
}
