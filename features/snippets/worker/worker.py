from temporalio.client import Client
from temporalio.worker import Worker


async def run():
    client = await Client.connect(
        "localhost:7233",
    )
    # @@@SNIPSTART python-worker-max-cached-workflows
    worker = Worker(client, task_queue="task-queue", max_cached_workflows=0)
    # @@@SNIPEND
    await worker.run()
