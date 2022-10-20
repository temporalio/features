from __future__ import annotations

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self):
        self.be_done = False

    @workflow.run
    async def run(self) -> None:
        await workflow.wait_condition(lambda: self.be_done)
        return None

    @workflow.query(name="q")
    def string_q(self) -> str:
        return "hi bob"

    @workflow.signal(name="finish")
    def finish_sig(self):
        self.be_done = True


@workflow.defn
class FakeWf:
    @workflow.run
    async def run(self) -> None:
        return None

    @workflow.query(name="q")
    def num_q(self) -> int:
        return 1


async def checker(_: Runner, handle: WorkflowHandle):
    q1 = await handle.query(FakeWf.num_q)
    # Python (at the moment) doesn't detect any type mismatch here
    assert q1 == "hi bob"
    await handle.signal(Workflow.finish_sig)
    await handle.result()


register_feature(workflows=[Workflow], check_result=checker)
