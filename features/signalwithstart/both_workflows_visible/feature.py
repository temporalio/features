import uuid
from dataclasses import dataclass
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

WORKFLOW_INPUT = "workflow-input"
SIGNAL_VALUE = "signal-input"
MEMO_KEY = "memo-key"
MEMO_VALUE = "memo-value"


@dataclass
class SwsResult:
    workflow_id: str
    run_id: str


@workflow.defn
class TargetWorkflow:
    def __init__(self) -> None:
        self._signal_value: Optional[str] = None

    @workflow.run
    async def run(self, _input: str) -> str:
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
        handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            WORKFLOW_INPUT,
            id=target_id,
            task_queue=workflow.info().task_queue,
            signal=TargetWorkflow.my_signal,
            signal_args=SIGNAL_VALUE,
            memo={MEMO_KEY: MEMO_VALUE},
        )
        return SwsResult(workflow_id=handle.id, run_id=handle.run_id or "")


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"signalwithstart-both-visible-target-{uuid.uuid4()}"
    return await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-both-visible-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # Caller completes and returns the started target's identity.
    result: SwsResult = await handle.result()
    assert result.run_id, "expected a non-empty target run id"

    # Target completes after receiving the signal and returns the signal value.
    target = runner.client.get_workflow_handle(result.workflow_id, run_id=result.run_id)
    target_result = await target.result()
    assert target_result == SIGNAL_VALUE, (
        f"expected target to return {SIGNAL_VALUE!r}, got {target_result!r}"
    )

    # The memo passed in the signal-with-start request is visible on the target.
    desc = await target.describe()
    memo = await desc.memo()
    assert memo.get(MEMO_KEY) == MEMO_VALUE, (
        f"expected memo {MEMO_KEY}={MEMO_VALUE}, got {memo}"
    )


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
