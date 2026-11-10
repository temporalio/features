use crate::harness::{Feature, FeatureContext, FeatureFuture, FeatureOutcome};
use anyhow::{Result, ensure};
use temporalio_client::{WorkflowGetResultOptions, WorkflowSignalOptions};
use temporalio_macros::{workflow, workflow_methods};
use temporalio_sdk::{
    SyncWorkflowContext, WorkerOptions, WorkflowContext, WorkflowResult,
    workflow_replayer::WorkflowReplayerOptions,
};

#[workflow]
#[derive(Default)]
pub struct Workflow {
    state: String,
}

#[workflow_methods]
impl Workflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<String> {
        ctx.wait_condition(|s| !s.state.is_empty()).await?;
        Ok(ctx.state(|s| s.state.clone()))
    }

    #[signal]
    pub fn my_signal(&mut self, _ctx: &mut SyncWorkflowContext<Self>, arg: String) {
        self.state = arg;
    }
}

struct BasicSignal;

impl Feature for BasicSignal {
    fn worker_options(&self, ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(Some(
            WorkerOptions::new(ctx.task_queue.clone())
                .register_workflow::<Workflow>()?
                .build(),
        ))
    }

    fn replay_options(&self, ctx: &FeatureContext) -> Result<Option<WorkflowReplayerOptions>> {
        Ok(Some(ctx.replay_options::<Workflow>()?))
    }

    fn execute<'a>(&'a self, ctx: &'a FeatureContext) -> FeatureFuture<'a> {
        Box::pin(async move {
            let handle = ctx
                .client
                .start_workflow(Workflow::run, (), ctx.start_options())
                .await?;
            handle
                .signal(
                    Workflow::my_signal,
                    "arg".to_owned(),
                    WorkflowSignalOptions::default(),
                )
                .await?;
            ensure!(
                handle.get_result(WorkflowGetResultOptions::default()).await? == "arg",
                "signal did not mutate workflow state"
            );
            Ok(FeatureOutcome::Passed)
        })
    }
}

pub fn feature() -> Box<dyn Feature> {
    Box::new(BasicSignal)
}
