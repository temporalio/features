import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle, WorkflowUpdateFailedError
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    counter = 0

    @workflow.run
    async def run(self) -> int:
        await workflow.wait_condition(lambda: self.counter == 5)
        return self.counter

    @workflow.update
    async def do_update(self, arg: int) -> int:
        self.counter += arg
        return self.counter

    @do_update.validator
    def reject_negatives(self, arg: int):
        if arg < 0:
            raise ApplicationError("I *HATE* negative numbers!")


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    for i in range(5):
        try:
            await handle.execute_update(Workflow.do_update, -1)
            raise RuntimeError("Should throw")
        except WorkflowUpdateFailedError as err:
            pass

        await handle.execute_update(Workflow.do_update, 1)

    assert 5 == await handle.result()

    # Verify no rejections were written to history since we failed in the validator
    async for e in handle.fetch_history_events():
        if e.HasField("workflow_execution_update_rejected_event_attributes"):
            raise RuntimeError("There shouldn't have been a rejected event!")


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
)
