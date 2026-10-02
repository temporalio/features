from __future__ import annotations

from datetime import timedelta

from temporalio import activity, workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowFailureError, WorkflowHandle
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

ACTIVITY_ERROR_MESSAGE = "activity failed"
WORKFLOW_ERROR_MESSAGE = "workflow failed"
# Python only puts an activity ID in the workflow side context when the
# workflow sets one explicitly.
ACTIVITY_ID = "ser-ctx-activity"


@activity.defn
async def failing_activity() -> None:
    raise ApplicationError(ACTIVITY_ERROR_MESSAGE, type="ActivityError")


@workflow.defn
class Workflow:
    """Lets an activity fail and then fails itself, so that both an activity
    scoped and a workflow scoped failure conversion are recorded."""

    @workflow.run
    async def run(self) -> None:
        try:
            await workflow.execute_activity(
                failing_activity,
                activity_id=ACTIVITY_ID,
                start_to_close_timeout=timedelta(seconds=10),
                retry_policy=RetryPolicy(maximum_attempts=1),
            )
        except Exception:
            raise ApplicationError(WORKFLOW_ERROR_MESSAGE, type="WorkflowError")


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    try:
        await handle.result()
        raise AssertionError("expected the workflow to fail")
    except WorkflowFailureError as err:
        assert WORKFLOW_ERROR_MESSAGE in str(err.cause)

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

    activity_failed = sercontext.find_event(
        events,
        "ActivityTaskFailed",
        lambda e: e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_FAILED,
    ).activity_task_failed_event_attributes
    assert activity_failed.failure.source == sercontext.activity_signature(
        runner.namespace,
        handle.id,
        started.workflow_type.name,
        scheduled.activity_type.name,
        scheduled.activity_id,
        scheduled.task_queue.name,
        False,
    )

    workflow_failed = sercontext.find_event(
        events,
        "WorkflowExecutionFailed",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED,
    ).workflow_execution_failed_event_attributes
    assert workflow_failed.failure.source == sercontext.workflow_signature(
        runner.namespace, handle.id
    )


register_feature(
    workflows=[Workflow],
    activities=[failing_activity],
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
