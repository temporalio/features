from __future__ import annotations

from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        child_wf = await workflow.start_child_workflow(
            ChildWorkflow.run, "child-wf-arg"
        )
        await child_wf.signal(ChildWorkflow.my_signal, "signal-arg")
        return await child_wf


@workflow.defn
class ChildWorkflow:
    def __init__(self) -> None:
        self._state = ""

    @workflow.run
    async def run(self, input: str) -> str:
        await workflow.wait_condition(lambda: self._state != "")
        return f"{input} {self._state}"

    @workflow.signal
    async def my_signal(self, arg: str):
        self._state = arg


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.start_parameterless_workflow(Workflow)


register_feature(
    workflows=[Workflow, ChildWorkflow],
    expect_run_result=f"child-wf-arg signal-arg",
    start=start,
)
