import uuid
from dataclasses import dataclass
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

SIGNAL_VALUE = "test-signal-value"


@dataclass
class SwsResult:
    """What the caller workflow returns: the identity of the workflow that the
    signal-with-start operation started (or signaled)."""

    workflow_id: str
    run_id: str


@workflow.defn
class TargetWorkflow:
    def __init__(self) -> None:
        self._signal_value: Optional[str] = None

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._signal_value is not None)
        assert self._signal_value is not None
        return self._signal_value

    @workflow.signal
    def my_signal(self, value: str) -> None:
        self._signal_value = value


@workflow.defn
class CallerWorkflow:
    @workflow.run
    async def run(self, target_id: str) -> SwsResult:
        # Signal-with-start the target from inside this workflow. With no
        # existing workflow for target_id this starts a brand-new execution and
        # delivers the signal to it.
        handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            id=target_id,
            task_queue=workflow.info().task_queue,
            signal=TargetWorkflow.my_signal,
            signal_args=SIGNAL_VALUE,
        )
        return SwsResult(workflow_id=handle.id, run_id=handle.run_id or "")


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"signalwithstart-happy-path-target-{uuid.uuid4()}"
    return await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-happy-path-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    result: SwsResult = await handle.result()
    assert result.run_id, "expected a non-empty run id for the started target workflow"

    # The signal-with-start created a new target execution; it completes after
    # receiving the signal and returns the signal value.
    target = runner.client.get_workflow_handle(result.workflow_id, run_id=result.run_id)
    target_result = await target.result()
    assert target_result == SIGNAL_VALUE, (
        f"expected target to return {SIGNAL_VALUE!r}, got {target_result!r}"
    )


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
