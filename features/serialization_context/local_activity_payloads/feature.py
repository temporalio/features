from __future__ import annotations

from datetime import timedelta

from temporalio import activity, workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

WORKFLOW_INPUT = "hello"
ACTIVITY_NAME = "local_activity"
# Python only puts an activity ID in the workflow side context when the
# workflow sets one explicitly.
ACTIVITY_ID = "ser-ctx-local-activity"


@activity.defn
async def local_activity(input: str) -> str:
    return f"{input}|local"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, input: str) -> str:
        return await workflow.execute_local_activity(
            local_activity,
            input,
            activity_id=ACTIVITY_ID,
            start_to_close_timeout=timedelta(seconds=10),
        )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == f"{WORKFLOW_INPUT}|local"

    events = await sercontext.events(handle)
    started = sercontext.find_event(
        events,
        "WorkflowExecutionStarted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
    ).workflow_execution_started_event_attributes

    # Local activity payloads never reach history, so the context is asserted on
    # what the codec was actually asked to convert with.
    prefix = (
        f"act|{runner.namespace}|{handle.id}|{started.workflow_type.name}"
        f"|{ACTIVITY_NAME}|{ACTIVITY_ID}"
    )
    suffix = f"|{started.task_queue.name}|True"
    assert any(
        s.startswith(prefix) and s.endswith(suffix)
        for s in sercontext.observed_signatures
    ), (
        f"no local activity context observed, wanted {prefix}...{suffix}, "
        f"got {sorted(sercontext.observed_signatures)}"
    )


register_feature(
    workflows=[Workflow],
    activities=[local_activity],
    start_options={"arg": WORKFLOW_INPUT},
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
