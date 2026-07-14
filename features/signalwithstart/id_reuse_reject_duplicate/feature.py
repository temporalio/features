import uuid
from dataclasses import dataclass
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle
from temporalio.common import WorkflowIDReusePolicy

from harness.python.feature import Runner, register_feature

SIGNAL_VALUE = "test-signal-value"


@dataclass
class SwsResult:
    workflow_id: str
    run_id: str
    error: Optional[str] = None


def _error_chain(err: BaseException) -> str:
    """Flatten an exception and its causes into a single message. The useful
    detail (e.g. "already started") lives on the cause of the NexusOperationError,
    not its top-level message."""
    parts = []
    cur: Optional[BaseException] = err
    for _ in range(8):
        if cur is None:
            break
        parts.append(f"{type(cur).__name__}: {cur}")
        cur = getattr(cur, "cause", None) or cur.__cause__
    return " | ".join(parts)


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
        # With REJECT_DUPLICATE and a previously-completed run, the operation is
        # expected to fail. Capture the failure so the client can assert on it.
        try:
            handle = await workflow.signal_with_start_workflow(
                TargetWorkflow.run,
                id=target_id,
                task_queue=workflow.info().task_queue,
                signal=TargetWorkflow.my_signal,
                signal_args=SIGNAL_VALUE,
                id_reuse_policy=WorkflowIDReusePolicy.REJECT_DUPLICATE,
            )
            return SwsResult(workflow_id=handle.id, run_id=handle.run_id or "")
        except Exception as err:
            return SwsResult(workflow_id=target_id, run_id="", error=_error_chain(err))


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"signalwithstart-id-reuse-reject-dup-target-{uuid.uuid4()}"

    # Start and complete the target so a closed run exists for target_id.
    target = await runner.client.start_workflow(
        TargetWorkflow.run,
        id=target_id,
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    await target.signal(TargetWorkflow.my_signal, "setup")
    await target.result()

    return await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-id-reuse-reject-dup-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    result: SwsResult = await handle.result()
    assert result.error, "expected signal-with-start to fail with REJECT_DUPLICATE"
    assert "duplicate" in result.error.lower() or "already" in result.error.lower(), (
        f"unexpected failure message: {result.error}"
    )


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
