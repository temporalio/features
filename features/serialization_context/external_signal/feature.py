from __future__ import annotations

import uuid
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle

from features.serialization_context.sercontext import sercontext
from harness.python.feature import Runner, register_feature

SIGNAL_DATA = "signaled"


@workflow.defn
class Receiver:
    def __init__(self) -> None:
        self._received: Optional[str] = None

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._received is not None)
        assert self._received is not None
        return self._received

    @workflow.signal
    def external(self, data: str) -> None:
        self._received = data


@workflow.defn
class Workflow:
    """Signals another running workflow. The payload is serialized with the
    target's workflow ID, not this workflow's own ID."""

    @workflow.run
    async def run(self, target_id: str) -> str:
        await workflow.get_external_workflow_handle_for(
            Receiver.run, target_id
        ).signal(Receiver.external, SIGNAL_DATA)
        return target_id


def receiver_id(runner: Runner) -> str:
    return f"{runner.task_queue}_receiver"


async def start(runner: Runner) -> WorkflowHandle:
    receiver = await runner.client.start_workflow(
        Receiver.run,
        id=receiver_id(runner),
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )

    handle = await runner.client.start_workflow(
        Workflow.run,
        receiver.id,
        id=f"{runner.feature.rel_dir}-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )

    assert await receiver.result() == SIGNAL_DATA
    return handle


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert await handle.result() == receiver_id(runner)

    expected = sercontext.workflow_signature(runner.namespace, receiver_id(runner))
    assert expected != sercontext.workflow_signature(runner.namespace, handle.id)

    sender_events = await sercontext.events(handle)
    initiated = sercontext.find_event(
        sender_events,
        "SignalExternalWorkflowExecutionInitiated",
        lambda e: e.event_type
        == EventType.EVENT_TYPE_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_INITIATED,
    ).signal_external_workflow_execution_initiated_event_attributes
    assert sercontext.first_signature(initiated.input) == expected

    receiver_events = await sercontext.events(
        runner.client.get_workflow_handle(receiver_id(runner))
    )
    signaled = sercontext.find_event(
        receiver_events,
        "WorkflowExecutionSignaled",
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED,
    ).workflow_execution_signaled_event_attributes
    assert sercontext.first_signature(signaled.input) == expected


register_feature(
    workflows=[Workflow, Receiver],
    start=start,
    check_result=check_result,
    data_converter=sercontext.data_converter(),
)
