from __future__ import annotations

import asyncio
from datetime import timedelta
from typing import Optional

from temporalio import activity, workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.converter import ActivitySerializationContext

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

ACTIVITY_RESULT = "completed-out-of-band"
HEARTBEAT_DATA = "beat"
# Python only puts an activity ID in the workflow side context when the
# workflow sets one explicitly.
ACTIVITY_ID = "ser-ctx-activity"


class Scheduled:
    """What the activity worker saw, used by the completing client to
    reconstruct the same activity serialization context."""

    info: Optional[activity.Info] = None


@activity.defn
async def pending_activity() -> str:
    Scheduled.info = activity.info()
    activity.raise_complete_async()


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        return await workflow.execute_activity(
            pending_activity,
            activity_id=ACTIVITY_ID,
            start_to_close_timeout=timedelta(minutes=1),
            heartbeat_timeout=timedelta(seconds=30),
        )


async def start(runner: Runner) -> WorkflowHandle:
    handle = await runner.start_single_parameterless_workflow()

    for _ in range(300):
        if Scheduled.info is not None:
            break
        await asyncio.sleep(0.1)
    info = Scheduled.info
    assert info is not None, "activity was never started"
    assert info.workflow_id is not None

    # Without an explicit context the client has no workflow ID or activity type
    # to build an activity serialization context from.
    async_handle = runner.client.get_async_activity_handle(
        workflow_id=info.workflow_id,
        run_id=info.workflow_run_id,
        activity_id=info.activity_id,
    ).with_context(
        ActivitySerializationContext(
            namespace=runner.namespace,
            workflow_id=info.workflow_id,
            workflow_type=info.workflow_type,
            activity_type=info.activity_type,
            activity_id=info.activity_id,
            activity_task_queue=runner.task_queue,
            is_local=False,
        )
    )
    await async_handle.heartbeat(HEARTBEAT_DATA)
    await async_handle.complete(ACTIVITY_RESULT)
    return handle


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == ACTIVITY_RESULT

    info = Scheduled.info
    assert info is not None

    events = await sercontext.events(handle)
    completed = sercontext.find_event(
        events,
        "ActivityTaskCompleted",
        lambda e: e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_COMPLETED,
    ).activity_task_completed_event_attributes
    assert sercontext.first_signature(completed.result) == sercontext.activity_signature(
        runner.namespace,
        info.workflow_id,
        info.workflow_type,
        info.activity_type,
        info.activity_id,
        runner.task_queue,
        False,
    )


register_feature(
    workflows=[Workflow],
    activities=[pending_activity],
    start=start,
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
