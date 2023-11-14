import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

activity_started = asyncio.Semaphore(value=0)
finish_activity = asyncio.Semaphore(value=0)


@workflow.defn
class Workflow:
    am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_activities(self):
        await workflow.execute_activity(
            blocks, start_to_close_timeout=timedelta(minutes=1)
        )
        self.am_done = True


@activity.defn
async def blocks() -> str:
    activity_started.release()
    await finish_activity.acquire()
    return "hi"


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # In a task right now since no true async updates
    update_task = asyncio.create_task(handle.execute_update(Workflow.do_activities))

    await activity_started.acquire()
    await runner.stop_worker()
    runner.start_worker()
    finish_activity.release()

    await update_task
    await handle.result()


register_feature(
    workflows=[Workflow],
    activities=[blocks],
    check_result=check_result,
    start=start,
)
