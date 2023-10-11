from datetime import timedelta

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

WORKFLOW_INITIAL_STATE = "1"
SIGNAL_ARG = "2"


@workflow.defn(name="MyWorkflow")
class Workflow:
    def __init__(self) -> None:
        self._state = WORKFLOW_INITIAL_STATE

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._state != WORKFLOW_INITIAL_STATE)
        return self._state

    @workflow.signal
    async def my_signal(self, arg: str):
        self._state = arg


async def start(runner: Runner) -> WorkflowHandle:
    handle = await runner.client.start_workflow(
        "MyWorkflow",
        id="workflow-id",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    await handle.signal("my_signal", SIGNAL_ARG)
    return handle


register_feature(
    workflows=[Workflow],
    expect_run_result=SIGNAL_ARG,
    start=start,
)
