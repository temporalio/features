use crate::harness::{Feature, FeatureContext, FeatureFuture, FeatureOutcome};
use anyhow::{Result, ensure};
use std::time::Duration;
use temporalio_client::{
    ActivityDescribeOptions, ActivityExecutionInfoLike, ActivityExecutionStatus,
    ActivityStartOptions,
};
use temporalio_macros::activities;
use temporalio_sdk::{
    WorkerOptions,
    activities::{ActivityContext, ActivityError},
};

pub struct Activities;

#[activities]
impl Activities {
    #[activity]
    pub async fn echo(_ctx: ActivityContext, value: String) -> Result<String, ActivityError> {
        Ok(value)
    }
}

struct StandaloneActivity;

impl Feature for StandaloneActivity {
    fn worker_options(&self, ctx: &FeatureContext) -> Result<Option<WorkerOptions>> {
        Ok(Some(
            WorkerOptions::new(ctx.task_queue.clone())
                .register_activities(Activities)
                .build(),
        ))
    }

    fn execute<'a>(&'a self, ctx: &'a FeatureContext) -> FeatureFuture<'a> {
        Box::pin(async move {
            let options = ActivityStartOptions::with_start_to_close_timeout(
                ctx.task_queue.clone(),
                ctx.task_queue.clone(),
                Duration::from_secs(60),
            )
            .build();
            let handle = ctx
                .client
                .start_activity(Activities::echo, "hello".to_owned(), options)
                .await?;
            ensure!(
                handle.result().await? == "hello",
                "unexpected activity result"
            );
            let description = handle.describe(ActivityDescribeOptions::default()).await?;
            ensure!(
                description.status() == ActivityExecutionStatus::Completed,
                "unexpected activity status"
            );
            Ok(FeatureOutcome::Passed)
        })
    }
}

pub fn feature() -> Box<dyn Feature> {
    Box::new(StandaloneActivity)
}
