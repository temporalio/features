from __future__ import annotations

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self):
        self.counter = 0
        self.be_done = False

    @workflow.run
    async def run(self) -> None:
        await workflow.wait_condition(lambda: self.be_done)
        return None

    @workflow.query(name="get_counter")
    def counter_q(self):
        return self.counter

    @workflow.signal(name="inc_counter")
    def counter_sig(self):
        self.counter += 1

    @workflow.signal(name="finish")
    def finish_sig(self):
        self.be_done = True


async def checker(_: Runner, handle: WorkflowHandle):
    q1 = await handle.query(Workflow.counter_q)
    assert q1 == 0
    await handle.signal(Workflow.counter_sig)
    q2 = await handle.query(Workflow.counter_q)
    assert q2 == 1
    await handle.signal(Workflow.counter_sig)
    await handle.signal(Workflow.counter_sig)
    await handle.signal(Workflow.counter_sig)
    q3 = await handle.query(Workflow.counter_q)
    assert q3 == 4
    await handle.signal(Workflow.finish_sig)
    await handle.result()


register_feature(workflows=[Workflow], check_result=checker)
