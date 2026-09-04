use anyhow::{Result, anyhow, ensure};
use std::time::Duration;
use temporalio_client::{
    WorkflowGetResultOptions, WorkflowStartOptions, errors::WorkflowGetResultError,
};
use temporalio_common::{RetryPolicy, error::IncomingError};
use temporalio_features_harness::{Feature, FeatureContext, async_trait};
use temporalio_macros::{activities, workflow, workflow_methods};
use temporalio_sdk::{
    ActivityOptions, ApplicationFailure, WorkerOptions, WorkflowContext, WorkflowResult,
    activities::{ActivityContext, ActivityError},
};

#[workflow]
#[derive(Default)]
pub struct RetryOnErrorWorkflow;

#[workflow_methods]
impl RetryOnErrorWorkflow {
    #[run]
    pub async fn run(ctx: &mut WorkflowContext<Self>) -> WorkflowResult<()> {
        #[allow(deprecated)]
        ctx.start_activity(
            RetryOnErrorActivities::always_fail,
            (),
            ActivityOptions::with_schedule_to_close_timeout(Duration::from_secs(60))
                .retry_policy(
                    RetryPolicy::builder()
                        .initial_interval(Duration::from_millis(1))
                        .backoff_coefficient(1.0)
                        .maximum_attempts(5)
                        .build(),
                )
                .build(),
        )
        .await?;
        Ok(())
    }
}

pub struct RetryOnErrorActivities;

#[activities]
impl RetryOnErrorActivities {
    #[activity]
    pub async fn always_fail(ctx: ActivityContext) -> Result<(), ActivityError> {
        Err(ActivityError::application(ApplicationFailure::new(
            anyhow!("activity attempt {} failed", ctx.info().attempt),
        )))
    }
}

struct RetryOnErrorFeature;

#[async_trait]
impl Feature for RetryOnErrorFeature {
    fn worker_options(&self, mut worker_options: WorkerOptions) -> Result<WorkerOptions> {
        worker_options
            .register_workflow::<RetryOnErrorWorkflow>()?
            .register_activities(RetryOnErrorActivities);
        Ok(worker_options)
    }

    async fn execute(&self, context: FeatureContext) -> Result<()> {
        let handle = context
            .client
            .start_workflow(
                RetryOnErrorWorkflow::run,
                (),
                WorkflowStartOptions::new(context.task_queue, context.workflow_id)
                    .execution_timeout(Duration::from_secs(60))
                    .build(),
            )
            .await?;
        let error = handle
            .get_result(WorkflowGetResultOptions::default())
            .await
            .expect_err("workflow should fail after activity retries");
        let WorkflowGetResultError::Failed(failure) = error else {
            return Err(anyhow!("expected activity failure, got {error}"));
        };
        let IncomingError::Activity(err) = failure.as_ref() else {
            return Err(anyhow!("expected activity failure, got {failure}"));
        };
        let Some(IncomingError::Application(cause)) = err.cause() else {
            return Err(anyhow!(
                "expected application failure cause, got {:?}",
                err.cause()
            ));
        };
        ensure!(
            cause.source_error().to_string() == "activity attempt 5 failed",
            "unexpected activity failure: {}",
            cause.source_error()
        );
        Ok(())
    }
}

pub fn feature() -> impl Feature {
    RetryOnErrorFeature
}
