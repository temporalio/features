import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_activities(self):
        act_futures = []
        for i in range(6):
            act_futures.append(
                workflow.start_activity(
                    say_hi, start_to_close_timeout=timedelta(minutes=1)
                )
            )
        results = await asyncio.gather(*act_futures)
        self.am_done = True
        return len(results)


@activity.defn
async def say_hi() -> str:
    return "hi"


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    await handle.execute_update(Workflow.do_activities)
    await handle.result()


register_feature(
    workflows=[Workflow],
    activities=[say_hi],
    check_result=check_result,
    start=start,
)
