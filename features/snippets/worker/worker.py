import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import Client
from temporalio.common import VersioningBehavior, WorkerDeploymentVersion
from temporalio.worker import Worker, WorkerDeploymentConfig


@activity.defn
async def some_activity() -> None:
    return None


@workflow.defn
class HelloWorkflow:
    @workflow.run
    async def run(self, name: str) -> str:
        return f"Hello, {name}!"


async def run():
    client = await Client.connect(
        "localhost:7233",
    )
    # @@@SNIPSTART python-worker-max-cached-workflows
    worker = Worker(client, task_queue="task-queue", max_cached_workflows=0)
    # @@@SNIPEND
    await worker.run()


async def run_worker():
    # @@@SNIPSTART python-create-worker
    client = await Client.connect("localhost:7233")

    worker = Worker(
        client,
        task_queue="my-task-queue",
        workflows=[HelloWorkflow],
        activities=[some_activity],
    )
    await worker.run()
    # @@@SNIPEND


async def run_versioned_worker():
    client = await Client.connect("localhost:7233")

    # @@@SNIPSTART python-versioned-worker
    worker = Worker(
        client,
        task_queue="my-task-queue",
        workflows=[HelloWorkflow],
        activities=[some_activity],
        deployment_config=WorkerDeploymentConfig(
            version=WorkerDeploymentVersion(
                deployment_name="my-app",
                build_id="1.0",
            ),
            use_worker_versioning=True,
            default_versioning_behavior=VersioningBehavior.PINNED,
        ),
    )
    # @@@SNIPEND
    await worker.run()


async def run_worker_until_interrupted(interrupt_event: asyncio.Event):
    client = await Client.connect("localhost:7233")

    # @@@SNIPSTART python-worker-graceful-shutdown
    worker = Worker(
        client,
        task_queue="my-task-queue",
        workflows=[HelloWorkflow],
        activities=[some_activity],
        graceful_shutdown_timeout=timedelta(seconds=30),
    )
    async with worker:
        await interrupt_event.wait()
    # @@@SNIPEND
