import asyncio
from datetime import timedelta
from typing import Optional

from temporalio import activity, workflow
from temporalio.client import Client, WorkflowHandle
from temporalio.common import RetryPolicy
from temporalio.exceptions import ActivityError, ApplicationError, CancelledError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self._activity_result: Optional[str] = None

    @workflow.run
    async def run(self) -> None:
        # Start workflow
        handle = workflow.start_activity(
            cancellable_activity,
            schedule_to_close_timeout=timedelta(minutes=1),
            heartbeat_timeout=timedelta(seconds=5),
            # Disable retry
            retry_policy=RetryPolicy(maximum_attempts=1),
            cancellation_type=workflow.ActivityCancellationType.TRY_CANCEL,
        )

        # Sleep for very short time (force task turnover)
        await asyncio.sleep(0.01)

        # Cancel and confirm the activity errors with the cancel
        handle.cancel()
        try:
            await handle
            raise ApplicationError("No error")
        except ActivityError as err:
            if not isinstance(err.cause, CancelledError):
                raise ApplicationError("Expected activity cancel") from err

        # Confirm signal is cancelled
        await workflow.wait_condition(
            lambda: self._activity_result is not None, timeout=10
        )
        if self._activity_result != "cancelled":
            raise ApplicationError(f"Expected cancelled, got {self._activity_result}")

    @workflow.signal
    def activity_result(self, result: str) -> None:
        self._activity_result = result


client: Optional[Client] = None


@activity.defn
async def cancellable_activity() -> None:
    assert client

    # Heartbeat every second for a minute
    result = "timeout"
    try:
        for _ in range(0, 60):
            await asyncio.sleep(1)
            activity.heartbeat()
    except asyncio.CancelledError:
        result = "cancelled"

    # Send result as signal to workflow
    await client.get_workflow_handle_for(
        Workflow.run, activity.info().workflow_id
    ).signal(Workflow.activity_result, result)


async def start(runner: Runner) -> WorkflowHandle:
    global client
    client = runner.client
    return await runner.start_single_parameterless_workflow()


register_feature(
    workflows=[Workflow],
    activities=[cancellable_activity],
    start=start,
)
