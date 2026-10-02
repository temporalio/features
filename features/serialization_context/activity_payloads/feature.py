from __future__ import annotations

from datetime import timedelta

from temporalio import activity, workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

WORKFLOW_INPUT = "hello"
HEARTBEAT_DATA = "beat"
# Python only puts an activity ID in the workflow side context when the
# workflow sets one explicitly.
ACTIVITY_ID = "ser-ctx-activity"


@activity.defn
async def activity_with_heartbeat(input: str) -> str:
    """Fails its first attempt so the second one has to decode the heartbeat
    details recorded by the first."""
    if activity.info().attempt == 1:
        activity.heartbeat(HEARTBEAT_DATA)
        raise ApplicationError("retrying to read back heartbeat details")
    return f"{input}|{activity.info().heartbeat_details[0]}"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, input: str) -> str:
        return await workflow.execute_activity(
            activity_with_heartbeat,
            input,
            activity_id=ACTIVITY_ID,
            start_to_close_timeout=timedelta(seconds=10),
            heartbeat_timeout=timedelta(seconds=5),
            retry_policy=RetryPolicy(
                initial_interval=timedelta(milliseconds=1), maximum_attempts=2
            ),
        )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == f"{WORKFLOW_INPUT}|{HEARTBEAT_DATA}"

    events = await sercontext.events(handle)

    started = sercontext.find_event(
        events,
        "WorkflowExecutionStarted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
    ).workflow_execution_started_event_attributes
    scheduled = sercontext.find_event(
        events,
        "ActivityTaskScheduled",
        lambda e: e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED,
    ).activity_task_scheduled_event_attributes

    expected = sercontext.activity_signature(
        runner.namespace,
        handle.id,
        started.workflow_type.name,
        scheduled.activity_type.name,
        scheduled.activity_id,
        scheduled.task_queue.name,
        False,
    )
    assert sercontext.first_signature(scheduled.input) == expected

    completed = sercontext.find_event(
        events,
        "ActivityTaskCompleted",
        lambda e: e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_COMPLETED,
    ).activity_task_completed_event_attributes
    assert sercontext.first_signature(completed.result) == expected

    workflow_completed = sercontext.find_event(
        events,
        "WorkflowExecutionCompleted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED,
    ).workflow_execution_completed_event_attributes
    assert sercontext.first_signature(
        workflow_completed.result
    ) == sercontext.workflow_signature(runner.namespace, handle.id)


register_feature(
    workflows=[Workflow],
    activities=[activity_with_heartbeat],
    start_options={"arg": WORKFLOW_INPUT},
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
