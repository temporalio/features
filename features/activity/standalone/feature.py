import uuid
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import ActivityExecutionStatus, WorkflowHandle

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        pass


@activity.defn
async def echo(value: str) -> str:
    return value


async def start(runner: Runner) -> WorkflowHandle:
    handle = await runner.client.start_activity(
        echo,
        "hello",
        id=str(uuid.uuid4()),
        task_queue=runner.task_queue,
        start_to_close_timeout=timedelta(minutes=1),
    )
    assert await handle.result() == "hello"

    desc = await handle.describe()
    assert desc.status == ActivityExecutionStatus.COMPLETED

    return await runner.start_single_parameterless_workflow()


register_feature(
    workflows=[Workflow],
    activities=[echo],
    start=start,
)
