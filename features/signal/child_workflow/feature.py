from __future__ import annotations

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

CHILD_WORKFLOW_INITIAL_STATE = "1"
SIGNAL_ARG = "2"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        child_wf = await workflow.start_child_workflow(ChildWorkflow.run)
        await child_wf.signal("my_signal", SIGNAL_ARG)
        await child_wf
        return child_wf.result()


@workflow.defn
class ChildWorkflow:
    def __init__(self) -> None:
        self._state = CHILD_WORKFLOW_INITIAL_STATE

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(
            lambda: self._state != CHILD_WORKFLOW_INITIAL_STATE
        )
        return f"{input} {self._state}"

    @workflow.signal
    async def my_signal(self, arg: str):
        self._state = arg


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.start_parameterless_workflow(Workflow)


register_feature(
    workflows=[Workflow, ChildWorkflow],
    expect_run_result=SIGNAL_ARG,
    start=start,
)
