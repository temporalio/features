from __future__ import annotations

import dataclasses
from collections.abc import Sequence

import temporalio.converter
from temporalio import workflow
from temporalio.api.common.v1 import Payload
from temporalio.client import WorkflowHandle
from temporalio.converter import (
    ExternalStorage,
    StorageDriver,
    StorageDriverClaim,
    StorageDriverRetrieveContext,
    StorageDriverStoreContext,
)

from harness.python.feature import Runner, register_feature

# Exceed the server limit so the SDK must offload the result.
RESULT_SIZE = 3 * 1024 * 1024
STORAGE_THRESHOLD = 1024


class MemoryDriver(StorageDriver):
    def __init__(self) -> None:
        self.payloads: dict[str, bytes] = {}
        self.stores = 0
        self.retrieves = 0

    def name(self) -> str:
        return "query-result-memory"

    async def store(
        self,
        context: StorageDriverStoreContext,
        payloads: Sequence[Payload],
    ) -> list[StorageDriverClaim]:
        self.stores += 1
        claims = []
        for payload in payloads:
            key = f"payload-{len(self.payloads)}"
            self.payloads[key] = payload.SerializeToString()
            claims.append(StorageDriverClaim(claim_data={"key": key}))
        return claims

    async def retrieve(
        self,
        context: StorageDriverRetrieveContext,
        claims: Sequence[StorageDriverClaim],
    ) -> list[Payload]:
        self.retrieves += 1
        payloads = []
        for claim in claims:
            payload = Payload()
            payload.ParseFromString(self.payloads[claim.claim_data["key"]])
            payloads.append(payload)
        return payloads


driver = MemoryDriver()
data_converter = dataclasses.replace(
    temporalio.converter.default(),
    external_storage=ExternalStorage(
        drivers=[driver],
        payload_size_threshold=STORAGE_THRESHOLD,
    ),
)


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self.finished = False

    @workflow.run
    async def run(self) -> None:
        await workflow.wait_condition(lambda: self.finished)

    @workflow.query
    def oversized_result(self) -> str:
        return "a" * RESULT_SIZE

    @workflow.signal
    def finish(self) -> None:
        self.finished = True


async def check_result(_: Runner, handle: WorkflowHandle) -> None:
    result = await handle.query(Workflow.oversized_result)
    assert result == "a" * RESULT_SIZE
    assert driver.stores > 0
    assert driver.retrieves > 0

    await handle.signal(Workflow.finish)
    await handle.result()


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    data_converter=data_converter,
)
