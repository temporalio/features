import dataclasses

from temporalio.client import Client, ClientConfig
from temporalio.converter import DataConverter, ExternalStorage
from temporalio.worker import Worker


async def setup(driver):
    # @@@SNIPSTART python-s3-external-storage-setup
    data_converter = dataclasses.replace(
        DataConverter.default,
        external_storage=ExternalStorage(drivers=[driver]),
    )

    client_config = ClientConfig.load_client_connect_config()

    client = await Client.connect(**client_config, data_converter=data_converter)

    worker = Worker(
        client,
        task_queue="my-task-queue",
        workflows=[],
        activities=[],
    )
    # @@@SNIPEND
    await worker.run()
