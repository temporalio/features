from __future__ import annotations

import asyncio

from temporalio import workflow
from temporalio.client import WorkflowHandle
from temporalio.service import RPCError, RPCStatusCode

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self):
        self.be_done = False

    @workflow.run
    async def run(self) -> None:
        await workflow.wait_condition(lambda: self.be_done)
        return None

    @workflow.query
    def simple_query(self):
        return True

    @workflow.signal(name="finish")
    def finish_sig(self):
        self.be_done = True


async def check_result(runner: Runner, handle: WorkflowHandle):
    # Stop worker
    await runner.stop_worker()
    try:
        # TODO: Override deadline once that's exposed
        await handle.query(Workflow.simple_query)
    except RPCError as e:
        # Cancelled rather than deadline exceeded since the timeout is client-side
        assert e.status == RPCStatusCode.CANCELLED
    # Restart the worker and finish the wf
    runner.start_worker()
    await handle.signal(Workflow.finish_sig)
    await runner.check_result(handle)


register_feature(workflows=[Workflow], check_result=check_result)
