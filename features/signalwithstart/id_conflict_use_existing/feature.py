import uuid
from dataclasses import dataclass
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle
from temporalio.common import WorkflowIDConflictPolicy

from harness.python.feature import Runner, register_feature

SIGNAL_VALUE = "test-signal-value"

_setup: dict = {}


@dataclass
class SwsResult:
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
        handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            id=target_id,
            task_queue=workflow.info().task_queue,
            signal=TargetWorkflow.my_signal,
            signal_args=SIGNAL_VALUE,
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        return SwsResult(workflow_id=handle.id, run_id=handle.run_id or "")


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"signalwithstart-id-conflict-use-existing-target-{uuid.uuid4()}"

    # Start the target and leave it running. USE_EXISTING should signal this run
    # and return its run id without starting a new one.
    target = await runner.client.start_workflow(
        TargetWorkflow.run,
        id=target_id,
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    _setup["original_run_id"] = target.result_run_id

    return await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-id-conflict-use-existing-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    result: SwsResult = await handle.result()
    assert result.run_id == _setup["original_run_id"], (
        f"expected existing run id {_setup['original_run_id']}, got {result.run_id}"
    )


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
