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
    """Flatten an exception and its causes; the useful detail lives on the cause
    of the NexusOperationError, not its top-level message."""
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
        try:
            handle = await workflow.signal_with_start_workflow(
                TargetWorkflow.run,
                id=target_id,
                task_queue=workflow.info().task_queue,
                signal=TargetWorkflow.my_signal,
                signal_args=SIGNAL_VALUE,
                id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE_FAILED_ONLY,
            )
            return SwsResult(workflow_id=handle.id, run_id=handle.run_id)
        except Exception as err:
            return SwsResult(workflow_id=target_id, run_id="", error=_error_chain(err))


async def _run_caller(runner: Runner, target_id: str) -> SwsResult:
    handle = await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-allow-dup-failed-only-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    return await handle.result()


async def start(runner: Runner) -> WorkflowHandle:
    # Sub-case 1: target completed successfully -> ALLOW_DUPLICATE_FAILED_ONLY
    # should reject the operation. start() drives this and returns its caller
    # handle; the remaining sub-case runs in check_result.
    target_id = f"signalwithstart-allow-dup-failed-only-completed-{uuid.uuid4()}"
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
        id=f"signalwithstart-allow-dup-failed-only-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # Sub-case 1: completed target -> operation fails.
    completed_result: SwsResult = await handle.result()
    assert (
        completed_result.error
    ), "expected failure for ALLOW_DUPLICATE_FAILED_ONLY against a completed run"

    # Sub-case 2: terminated target -> a new run should start.
    target_id = f"signalwithstart-allow-dup-failed-only-terminated-{uuid.uuid4()}"
    target = await runner.client.start_workflow(
        TargetWorkflow.run,
        id=target_id,
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    original_run_id = target.result_run_id
    await target.terminate("setup")

    terminated_result = await _run_caller(runner, target_id)
    assert (
        not terminated_result.error
    ), f"expected success after terminated run, got error: {terminated_result.error}"
    assert terminated_result.run_id, "expected a non-empty new run id"
    assert (
        terminated_result.run_id != original_run_id
    ), "expected a new run id after the previous run was terminated"

    # Cleanup the freshly-started run.
    try:
        await runner.client.get_workflow_handle(
            target_id, run_id=terminated_result.run_id
        ).terminate("cleanup")
    except Exception:
        pass


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
