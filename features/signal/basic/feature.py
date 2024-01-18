from datetime import timedelta

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self._state = ""

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._state != "")
        return self._state

    @workflow.signal
    async def my_signal(self, arg: str):
        self._state = arg


async def start(runner: Runner) -> WorkflowHandle:
    handle: WorkflowHandle = await runner.client.start_workflow(
        Workflow,
        id="workflow-id",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    await handle.signal(Workflow.my_signal, "arg")
    return handle


register_feature(
    workflows=[Workflow],
    expect_run_result="arg",
    start=start,
)
