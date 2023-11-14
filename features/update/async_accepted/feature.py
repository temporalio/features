import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import RPCError, WorkflowHandle, WorkflowUpdateFailedError
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    am_done = False
    proceed_signal = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_maybe_wait_update(self, sleep: bool) -> int:
        if sleep:
            await workflow.wait_condition(lambda: self.proceed_signal)
            self.proceed_signal = False
        else:
            raise ApplicationError("Dying on purpose")
        return 123

    @workflow.signal
    def finish(self):
        self.am_done = True

    @workflow.signal
    def unblock(self):
        self.proceed_signal = True


@activity.defn
async def say_hi() -> str:
    return "hi"


async def start(runner: Runner) -> WorkflowHandle:
    # TODO needs to check async specifically once we are using accepted wait policy in this test
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # Issue async update
    update_id = "sleepy_update"
    update_handle = await handle.start_update(
        Workflow.do_maybe_wait_update, True, id=update_id
    )
    await handle.signal(Workflow.unblock)
    # There's no API at the moment for directly creating a handle w/o calling start update since
    # async is still immature, also use that path if/when it exists.
    assert 123 == await update_handle.result()

    # Async update which throws
    fail_update_id = "failing_update"
    update_handle = await handle.start_update(
        Workflow.do_maybe_wait_update, False, id=fail_update_id
    )
    try:
        await update_handle.result()
        raise RuntimeError("Should have failed")
    except WorkflowUpdateFailedError as err:
        assert isinstance(err.cause, ApplicationError)
        assert "Dying on purpose" == err.cause.message

    # TODO: Python doesn't have an `Accepted` wait policy option yet. Add when it does.
    # Verify timeouts work, but we can only use RPC timeout for now, because of ☝️
    timeout_update_id = "timeout_update"
    update_handle = await handle.start_update(
        Workflow.do_maybe_wait_update, True, id=timeout_update_id
    )
    try:
        await update_handle.result(rpc_timeout=timedelta(seconds=1))
        raise RuntimeError("Should have failed")
    except RPCError as err:
        assert "Timeout expired" == err.message

    await handle.signal(Workflow.finish)
    await handle.result()


register_feature(
    workflows=[Workflow],
    activities=[say_hi],
    check_result=check_result,
    start=start,
)
