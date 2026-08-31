from __future__ import annotations

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

FINAL_RESULT = "done"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, remaining: int) -> str:
        if remaining > 0:
            workflow.continue_as_new(remaining - 1)
        return FINAL_RESULT


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    first_run_id = handle.first_execution_run_id
    assert await handle.result() == FINAL_RESULT

    # Continue-as-new keeps the workflow ID, so both runs share the context.
    expected = sercontext.workflow_signature(runner.namespace, handle.id)

    first_run_events = await sercontext.events(
        runner.client.get_workflow_handle(handle.id, run_id=first_run_id)
    )
    continued = sercontext.find_event(
        first_run_events,
        "WorkflowExecutionContinuedAsNew",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_CONTINUED_AS_NEW,
    ).workflow_execution_continued_as_new_event_attributes
    assert sercontext.first_signature(continued.input) == expected

    last_run_events = await sercontext.events(
        runner.client.get_workflow_handle(handle.id)
    )
    started = sercontext.find_event(
        last_run_events,
        "WorkflowExecutionStarted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
    ).workflow_execution_started_event_attributes
    assert sercontext.first_signature(started.input) == expected

    completed = sercontext.find_event(
        last_run_events,
        "WorkflowExecutionCompleted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED,
    ).workflow_execution_completed_event_attributes
    assert sercontext.first_signature(completed.result) == expected


register_feature(
    workflows=[Workflow],
    start_options={"arg": 1},
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
