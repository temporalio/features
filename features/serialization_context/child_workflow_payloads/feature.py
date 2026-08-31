from __future__ import annotations

import uuid
from datetime import timedelta

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

WORKFLOW_INPUT = "hello"
CHILD_ID_SUFFIX = "_child"
CHILD_RESULT_TAG = "|child"


@workflow.defn
class ChildWorkflow:
    @workflow.run
    async def run(self, input: str) -> str:
        return input + CHILD_RESULT_TAG


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, input: str) -> str:
        return await workflow.execute_child_workflow(
            ChildWorkflow.run,
            input,
            id=workflow.info().workflow_id + CHILD_ID_SUFFIX,
            run_timeout=timedelta(minutes=1),
        )


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.client.start_workflow(
        Workflow.run,
        WORKFLOW_INPUT,
        id=f"{runner.feature.rel_dir}-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == WORKFLOW_INPUT + CHILD_RESULT_TAG

    child_id = handle.id + CHILD_ID_SUFFIX
    # The child's payloads carry the child's own workflow ID, not the parent's.
    expected = sercontext.workflow_signature(runner.namespace, child_id)
    assert expected != sercontext.workflow_signature(runner.namespace, handle.id)

    parent_events = await sercontext.events(handle)
    initiated = sercontext.find_event(
        parent_events,
        "StartChildWorkflowExecutionInitiated",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_START_CHILD_WORKFLOW_EXECUTION_INITIATED,
    ).start_child_workflow_execution_initiated_event_attributes
    assert sercontext.first_signature(initiated.input) == expected

    child_completed = sercontext.find_event(
        parent_events,
        "ChildWorkflowExecutionCompleted",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_COMPLETED,
    ).child_workflow_execution_completed_event_attributes
    assert sercontext.first_signature(child_completed.result) == expected

    child_events = await sercontext.events(runner.client.get_workflow_handle(child_id))
    child_started = sercontext.find_event(
        child_events,
        "WorkflowExecutionStarted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
    ).workflow_execution_started_event_attributes
    assert sercontext.first_signature(child_started.input) == expected


register_feature(
    workflows=[Workflow, ChildWorkflow],
    start=start,
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
