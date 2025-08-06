import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import (
    ActivityError,
    ApplicationError,
    TimeoutError,
    TimeoutType,
)
from temporalio.worker import WorkerConfig

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        handle = workflow.start_activity(
            cancel_success,
            schedule_to_close_timeout=timedelta(milliseconds=300),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )
        handle1 = workflow.start_activity(
            cancel_failure,
            schedule_to_close_timeout=timedelta(milliseconds=300),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )
        handle2 = workflow.start_activity(
            cancel_ignore,
            schedule_to_close_timeout=timedelta(milliseconds=300),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )

        print("handle")
        await handle

        try:
            print("handle1")
            await handle1
            raise ApplicationError(
                "expected activity to fail with 'worker is shutting down'"
            )
        except ActivityError as err:
            if (
                not isinstance(err.cause, ApplicationError)
                or err.cause.message != "worker is shutting down"
            ):
                print("this shouldn't print")
                raise ApplicationError(
                    "expected activity to fail with 'worker is shutting down'"
                ) from err

        try:
            print("handle2")
            await handle2
            raise ApplicationError(
                "expected activity to fail with ScheduleToClose timeout"
            )
        except ActivityError as err:
            if (
                not isinstance(err.cause, TimeoutError)
                or err.cause.type != TimeoutType.SCHEDULE_TO_CLOSE
            ):
                raise ApplicationError(
                    "expected activity to fail with ScheduleToClose timeout"
                ) from err

        return "done"


@activity.defn
async def cancel_success() -> None:
    await activity.wait_for_worker_shutdown()


@activity.defn
async def cancel_failure() -> None:
    await activity.wait_for_worker_shutdown()
    raise ApplicationError("worker is shutting down")


@activity.defn
async def cancel_ignore() -> None:
    await asyncio.sleep(15)


async def start(runner: Runner):
    handle = await runner.start_single_parameterless_workflow()
    await asyncio.sleep(0.1)
    await runner.stop_worker()
    runner.start_worker()
    return handle


register_feature(
    workflows=[Workflow],
    activities=[cancel_success, cancel_failure, cancel_ignore],
    start=start,
    expect_run_result="done",
    worker_config=WorkerConfig(graceful_shutdown_timeout=timedelta(seconds=1)),
)