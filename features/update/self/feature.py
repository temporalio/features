import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import Client, WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self.am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.execute_activity(
            self_update, start_to_close_timeout=timedelta(seconds=1)
        )
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def update_me(self):
        self.am_done = True


client: Client


@activity.defn
async def self_update():
    global client
    assert client is not None
    handle = client.get_workflow_handle(activity.info().workflow_id)
    await handle.execute_update(Workflow.update_me)


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    global client
    client = runner.client
    return await runner.start_single_parameterless_workflow()


register_feature(
    workflows=[Workflow],
    activities=[self_update],
    start=start,
)
