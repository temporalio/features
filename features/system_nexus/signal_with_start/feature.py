from datetime import timedelta
from uuid import uuid4

from temporalio import workflow
from temporalio.client import WorkflowHandle
from temporalio.common import WorkflowIDConflictPolicy
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@workflow.defn
class TargetWorkflow:
    def __init__(self) -> None:
        self.signals: list[str] = []

    @workflow.run
    async def run(self, value: str) -> list[str]:
        await workflow.wait_condition(lambda: len(self.signals) == 2)
        return [f"started: {value}", *self.signals]

    @workflow.signal
    def add(self, value: str) -> None:
        self.signals.append(f"signal: {value}")


@workflow.defn
class CallerWorkflow:
    @workflow.run
    async def run(self, target_id: str, task_queue: str) -> tuple[str, str]:
        first_handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            "start-value",
            id=target_id,
            task_queue=task_queue,
            signal=TargetWorkflow.add,
            signal_args="signal-one",
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        second_handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            "unused-start-value",
            id=target_id,
            task_queue=task_queue,
            signal=TargetWorkflow.add,
            signal_args="signal-two",
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )

        first_run_id = first_handle.run_id
        second_run_id = second_handle.run_id
        if first_run_id is None:
            raise ApplicationError(
                "first SignalWithStart returned no run ID", non_retryable=True
            )
        if second_run_id is None:
            raise ApplicationError(
                "second SignalWithStart returned no run ID", non_retryable=True
            )
        if first_run_id != second_run_id:
            raise ApplicationError(
                "SignalWithStart calls targeted different runs: "
                f"first={first_run_id!r} second={second_run_id!r}",
                non_retryable=True,
            )
        return target_id, first_run_id


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"system-nexus-signal-with-start-{runner.task_queue}"
    return await runner.client.start_workflow(
        CallerWorkflow.run,
        args=[target_id, runner.task_queue],
        id=f"{runner.feature.rel_dir}-{uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    target_id, target_run_id = await handle.result()
    target = runner.client.get_workflow_handle(target_id, run_id=target_run_id)
    assert await target.result() == [
        "started: start-value",
        "signal: signal-one",
        "signal: signal-two",
    ]


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
