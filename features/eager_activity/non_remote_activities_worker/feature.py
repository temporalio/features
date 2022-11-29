from datetime import timedelta

from temporalio import activity, workflow
from temporalio.exceptions import ActivityError, TimeoutError, TimeoutType
from temporalio.worker import WorkerConfig

from harness.python.feature import register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        # Run a workflow that schedules a single activity with short schedule-to-close timeout
        try:
            await workflow.execute_activity(
                dummy,
                # Pick a long enough timeout for busy CI but not too long to get feedback quickly
                schedule_to_close_timeout=timedelta(seconds=3),
            )
            raise RuntimeError("Expected activity to time out")
        except ActivityError as e:
            # Catch activity failure in the workflow, check that it is caused by schedule-to-start timeout
            if not isinstance(e.cause, TimeoutError):
                raise e

            assert isinstance(e.cause, TimeoutError)
            if e.cause.type != TimeoutType.SCHEDULE_TO_START:
                raise e


@activity.defn
async def dummy() -> None:
    pass


# Start a worker with activities registered and non-local activities disabled
register_feature(
    workflows=[Workflow],
    activities=[dummy],
    worker_config=WorkerConfig(no_remote_activities=True),
)
