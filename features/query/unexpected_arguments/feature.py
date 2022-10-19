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

    @workflow.query(name="the_query")
    def the_query(self, arg: int):
        return f"hi {arg}"

    @workflow.signal(name="finish")
    def finish_sig(self):
        self.be_done = True


async def checker(_: Runner, handle: WorkflowHandle):
    # Wrong type TODO: Should be rejected
    await handle.query(Workflow.the_query, True)

    # Extra arg
    try:
        await handle.query(Workflow.the_query, 123, True)
    except TypeError:
        pass
    else:
        raise Exception("Extra arg in query must fail")

    # Not enough arg
    try:
        await handle.query(Workflow.the_query)
    except RPCError:
        # TODO: Should be TypeError like other rejection
        pass
    else:
        raise Exception("Not enough args in query must fail")

    await handle.signal(Workflow.finish_sig)
    await handle.result()


register_feature(workflows=[Workflow], check_result=checker)
