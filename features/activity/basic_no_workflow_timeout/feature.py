from datetime import timedelta

from temporalio import activity, workflow

from harness.python.feature import register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        await workflow.execute_activity(
            echo,
            schedule_to_close_timeout=timedelta(minutes=1),
        )
        return await workflow.execute_activity(
            echo,
            start_to_close_timeout=timedelta(minutes=1),
        )


@activity.defn
async def echo() -> str:
    return "echo"


register_feature(
    workflows=[Workflow],
    activities=[echo],
    expect_activity_error="activity attempt 5 failed",
    start_options={"execution_timeout": None},
)
