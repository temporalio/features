import asyncio
import json
import os
import time
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.api.enums.v1 import EventType
from temporalio.common import RetryPolicy
from temporalio.worker import WorkerConfig

from harness.python.feature import Runner, register_feature

WORKFLOW_COUNT = 5
SHUTDOWN_TIMEOUT = 5


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        while True:
            await asyncio.sleep(0.02)
            await workflow.execute_activity(
                noop,
                schedule_to_close_timeout=timedelta(seconds=10),
                start_to_close_timeout=timedelta(seconds=5),
                retry_policy=RetryPolicy(maximum_attempts=1),
            )


@activity.defn
async def noop() -> None:
    return None


async def start(runner: Runner):
    handles = []
    for _ in range(WORKFLOW_COUNT):
        handles.append(await runner.start_single_parameterless_workflow())

    try:
        for handle in handles:
            await runner.wait_for_activity_task_scheduled(handle, timeout=10.0)

        start_time = time.monotonic()
        await runner.stop_worker()
        assert time.monotonic() - start_time <= SHUTDOWN_TIMEOUT

        if expect_worker_poll_complete_on_shutdown():
            for handle in handles:
                async for event in handle.fetch_history_events():
                    assert event.event_type not in (
                        EventType.EVENT_TYPE_WORKFLOW_TASK_FAILED,
                        EventType.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT,
                    )
    finally:
        for handle in handles:
            try:
                await handle.terminate("feature cleanup")
            except Exception:
                pass

    return None


async def check_result(runner: Runner, handle) -> None:
    pass


def expect_worker_poll_complete_on_shutdown() -> bool:
    capabilities_json = os.environ.get("FEATURE_NAMESPACE_CAPABILITIES")
    if not capabilities_json:
        raise RuntimeError("FEATURE_NAMESPACE_CAPABILITIES is required")
    capabilities = json.loads(capabilities_json)
    if "workerPollCompleteOnShutdown" not in capabilities:
        raise RuntimeError(
            "FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown"
        )
    return capabilities["workerPollCompleteOnShutdown"]


register_feature(
    workflows=[Workflow],
    activities=[noop],
    start=start,
    check_result=check_result,
    worker_config=WorkerConfig(graceful_shutdown_timeout=timedelta(seconds=10)),
)
