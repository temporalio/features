from __future__ import annotations

import uuid
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

WORKFLOW_INPUT = "input"
MEMO_KEY = "ser-ctx-memo"
MEMO_VALUE = "memo"
QUERY_ARG = "query-"
UPDATE_ARG = "-update"
SIGNAL_DATA = "signal"


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self._input = ""
        self._signaled: Optional[str] = None

    @workflow.run
    async def run(self, input: str) -> str:
        self._input = input
        await workflow.wait_condition(lambda: self._signaled is not None)
        return f"{input}|{self._signaled}"

    @workflow.signal
    def append(self, data: str) -> None:
        self._signaled = data

    @workflow.query
    def prefixed(self, prefix: str) -> str:
        return prefix + self._input

    @workflow.update
    def suffixed(self, suffix: str) -> str:
        return self._input + suffix


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()

    handle = await runner.client.start_workflow(
        Workflow.run,
        WORKFLOW_INPUT,
        id=f"{runner.feature.rel_dir}-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
        memo={MEMO_KEY: MEMO_VALUE},
    )

    assert await handle.query(Workflow.prefixed, QUERY_ARG) == QUERY_ARG + WORKFLOW_INPUT
    assert (
        await handle.execute_update(Workflow.suffixed, UPDATE_ARG)
        == WORKFLOW_INPUT + UPDATE_ARG
    )
    await handle.signal(Workflow.append, SIGNAL_DATA)
    return handle


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == f"{WORKFLOW_INPUT}|{SIGNAL_DATA}"

    events = await sercontext.events(handle)
    expected = sercontext.workflow_signature(runner.namespace, handle.id)

    started = sercontext.find_event(
        events,
        "WorkflowExecutionStarted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
    ).workflow_execution_started_event_attributes
    assert sercontext.first_signature(started.input) == expected
    assert sercontext.signature_of(started.memo.fields[MEMO_KEY]) == expected

    completed = sercontext.find_event(
        events,
        "WorkflowExecutionCompleted",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED,
    ).workflow_execution_completed_event_attributes
    assert sercontext.first_signature(completed.result) == expected

    signaled = sercontext.find_event(
        events,
        "WorkflowExecutionSignaled",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED,
    ).workflow_execution_signaled_event_attributes
    assert sercontext.first_signature(signaled.input) == expected

    accepted = sercontext.find_event(
        events,
        "WorkflowExecutionUpdateAccepted",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_ACCEPTED,
    ).workflow_execution_update_accepted_event_attributes
    assert sercontext.first_signature(accepted.accepted_request.input.args) == expected

    update_completed = sercontext.find_event(
        events,
        "WorkflowExecutionUpdateCompleted",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_COMPLETED,
    ).workflow_execution_update_completed_event_attributes
    assert sercontext.first_signature(update_completed.outcome.success) == expected


register_feature(
    workflows=[Workflow],
    start=start,
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
