from __future__ import annotations

from temporalio import workflow
from temporalio.client import WorkflowHandle, RPCError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self):
        self.be_done = False

    @workflow.run
    async def run(self) -> None:
        await workflow.wait_condition(lambda: self.be_done)
        return None

    @workflow.signal(name="finish")
    def finish_sig(self):
        self.be_done = True


async def checker(_: Runner, handle: WorkflowHandle):
    try:
        await handle.query("nonexistent")
    except RPCError:
        pass
    else:
        raise Exception("Query with nonexistent handler must fail")

    await handle.signal(Workflow.finish_sig)
    await handle.result()


register_feature(workflows=[Workflow], check_result=checker)
