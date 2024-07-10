import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self.am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_update(self) -> str:
        self.am_done = True
        return "updated"


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert "updated" == await handle.execute_update(Workflow.do_update)
    await handle.result()


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
)
