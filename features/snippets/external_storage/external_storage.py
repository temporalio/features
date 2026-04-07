"""Snippets for the External Storage Python SDK documentation."""

import dataclasses
import os
import uuid
from collections.abc import Sequence

import aioboto3

from temporalio.api.common.v1 import Payload
from temporalio.client import Client, ClientConfig
from temporalio.contrib.aws.s3driver import S3StorageDriver
from temporalio.contrib.aws.s3driver.aioboto3 import new_aioboto3_client
from temporalio.converter import (
    DataConverter,
    ExternalStorage,
    StorageDriver,
    StorageDriverClaim,
    StorageDriverRetrieveContext,
    StorageDriverStoreContext,
    StorageDriverWorkflowInfo,
)
from temporalio.worker import Worker


async def s3_setup():
    session = aioboto3.Session()
    async with session.client("s3") as s3_client:
        driver = S3StorageDriver(
            client=new_aioboto3_client(s3_client),
            bucket="my-temporal-payloads",
        )

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


# @@@SNIPSTART python-custom-storage-driver
class LocalDiskStorageDriver(StorageDriver):
    def __init__(self, store_dir: str = "/tmp/temporal-payload-store") -> None:
        self._store_dir = store_dir

    def name(self) -> str:
        return "local-disk"

    async def store(
        self,
        context: StorageDriverStoreContext,
        payloads: Sequence[Payload],
    ) -> list[StorageDriverClaim]:
        os.makedirs(self._store_dir, exist_ok=True)

        prefix = self._store_dir
        target = context.target
        if isinstance(target, StorageDriverWorkflowInfo) and target.id:
            prefix = os.path.join(self._store_dir, target.namespace, target.id)
            os.makedirs(prefix, exist_ok=True)

        claims = []
        for payload in payloads:
            key = f"{uuid.uuid4()}.bin"
            file_path = os.path.join(prefix, key)
            with open(file_path, "wb") as f:
                f.write(payload.SerializeToString())
            claims.append(StorageDriverClaim(data={"path": file_path}))
        return claims

    async def retrieve(
        self,
        context: StorageDriverRetrieveContext,
        claims: Sequence[StorageDriverClaim],
    ) -> list[Payload]:
        payloads = []
        for claim in claims:
            file_path = claim.data["path"]
            with open(file_path, "rb") as f:
                raw = f.read()
            payload = Payload()
            payload.ParseFromString(raw)
            payloads.append(payload)
        return payloads
# @@@SNIPEND
