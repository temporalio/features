from datetime import timedelta

from temporalio import activity, workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError

from harness.python.feature import register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        # Allow 4 retries with basically no backoff
        await workflow.execute_activity(
            always_fail_activity,
            schedule_to_close_timeout=timedelta(minutes=1),
            retry_policy=RetryPolicy(
                # Retry basically immediately
                initial_interval=timedelta(milliseconds=1),
                # Do not increase retry backoff each time
                backoff_coefficient=1,
                # 5 total maximum attempts
                maximum_attempts=5,
            ),
        )


@activity.defn
async def always_fail_activity() -> None:
    raise ApplicationError(f"activity attempt {activity.info().attempt} failed")


register_feature(
    workflows=[Workflow],
    activities=[always_fail_activity],
    expect_activity_error="activity attempt 5 failed",
)
