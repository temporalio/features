import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    counter = 0
    proceed_signal = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.counter == 2)
        return "Hello, World!"

    @workflow.update
    async def inc_counter(self):
        self.counter += 1
        # Verify that dedupe works pre-update-completion
        await workflow.wait_condition(lambda: self.proceed_signal)
        self.proceed_signal = False
        return self.counter

    @workflow.signal
    def unblock(self):
        self.proceed_signal = True


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    update_id = "incrementer"
    h1 = await handle.start_update(Workflow.inc_counter, id=update_id)
    h2 = await handle.start_update(Workflow.inc_counter, id=update_id)
    await handle.signal(Workflow.unblock)
    results = await asyncio.gather(h1.result(), h2.result())
    assert results[0] == 1
    assert results[1] == 1

    # This only needs to start to unblock the workflow
    await handle.start_update(Workflow.inc_counter)

    # There should be two accepted updates, and only one of them should be completed with the set id
    total_updates = 0
    async for e in handle.fetch_history_events():
        if e.HasField("workflow_execution_update_completed_event_attributes"):
            assert (
                e.workflow_execution_update_completed_event_attributes.meta.update_id
                == update_id
            )
        elif e.HasField("workflow_execution_update_accepted_event_attributes"):
            total_updates += 1

    assert total_updates == 2


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
)
