import asyncio
import json
from collections.abc import Sequence
from datetime import timedelta
from typing import Any

from temporalio import workflow
from temporalio.api.common.v1 import Payload
from temporalio.client import WorkflowHandle
from temporalio.common import WorkflowIDConflictPolicy
from temporalio.converter import (
    CompositePayloadConverter,
    DataConverter,
    DefaultPayloadConverter,
    EncodingPayloadConverter,
    ExternalStorage,
    JSONPlainPayloadConverter,
    PayloadCodec,
    SerializationContext,
    StorageDriver,
    StorageDriverClaim,
    StorageDriverRetrieveContext,
    StorageDriverStoreContext,
    WithSerializationContext,
    WorkflowSerializationContext,
)
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature

with workflow.unsafe.imports_passed_through():
    from features.system_nexus.signal_with_start.support import (
        ContextValue,
        serialization_records,
    )

SYSTEM_ENVELOPE = "__temporal_system_payload"
CONTEXT_MARKER = "test-signal-with-start-context"
CODEC_MARKER = "test-signal-with-start-codec"


class ContextPayloadConverter(EncodingPayloadConverter, WithSerializationContext):
    def __init__(
        self,
        records: list[tuple[str, str, str]],
        context: SerializationContext | None = None,
    ) -> None:
        self.records = records
        self.context = context

    @property
    def encoding(self) -> str:
        return "test-signal-with-start-context"

    def with_context(self, context: SerializationContext) -> "ContextPayloadConverter":
        return ContextPayloadConverter(self.records, context)

    def to_payload(self, value: Any) -> Payload | None:
        if not isinstance(value, ContextValue):
            return None
        assert isinstance(self.context, WorkflowSerializationContext)
        self.records.append(("converter-encode", value.label, self.context.workflow_id))
        payload = JSONPlainPayloadConverter().to_payload(value)
        assert payload is not None
        payload.metadata["encoding"] = self.encoding.encode()
        payload.metadata[CONTEXT_MARKER] = self.context.workflow_id.encode()
        return payload

    def from_payload(self, payload: Payload, type_hint: type | None = None) -> Any:
        assert isinstance(self.context, WorkflowSerializationContext)
        expected_id = payload.metadata[CONTEXT_MARKER].decode()
        assert self.context.workflow_id == expected_id
        value = JSONPlainPayloadConverter().from_payload(payload, ContextValue)
        assert isinstance(value, ContextValue)
        self.records.append(("converter-decode", value.label, self.context.workflow_id))
        return value


class ContextPayloadConverterSet(CompositePayloadConverter):
    def __init__(self) -> None:
        super().__init__(
            ContextPayloadConverter(serialization_records),
            *DefaultPayloadConverter.default_encoding_payload_converters,
        )


class ContextPayloadCodec(PayloadCodec, WithSerializationContext):
    def __init__(
        self,
        records: list[tuple[str, str, str]],
        context: SerializationContext | None = None,
    ) -> None:
        self.records = records
        self.context = context

    def with_context(self, context: SerializationContext) -> "ContextPayloadCodec":
        return ContextPayloadCodec(self.records, context)

    async def encode(self, payloads: Sequence[Payload]) -> list[Payload]:
        return self._visit("codec-encode", payloads)

    async def decode(self, payloads: Sequence[Payload]) -> list[Payload]:
        return self._visit("codec-decode", payloads)

    def _visit(self, operation: str, payloads: Sequence[Payload]) -> list[Payload]:
        for payload in payloads:
            assert SYSTEM_ENVELOPE not in payload.metadata
            if CONTEXT_MARKER in payload.metadata:
                assert isinstance(self.context, WorkflowSerializationContext)
                expected_id = payload.metadata[CONTEXT_MARKER].decode()
                assert self.context.workflow_id == expected_id
                value = JSONPlainPayloadConverter().from_payload(payload, ContextValue)
                assert isinstance(value, ContextValue)
                self.records.append((operation, value.label, expected_id))
                payload.metadata[CODEC_MARKER] = expected_id.encode()
        return list(payloads)


class RecordingStorageDriver(StorageDriver):
    def __init__(self) -> None:
        self.storage: dict[str, bytes] = {}
        self.stored_payloads: list[Payload] = []

    def name(self) -> str:
        return "signal-with-start-test-storage"

    async def store(
        self, context: StorageDriverStoreContext, payloads: Sequence[Payload]
    ) -> list[StorageDriverClaim]:
        _ = context
        entries: list[tuple[str, bytes]] = []
        for payload in payloads:
            assert SYSTEM_ENVELOPE not in payload.metadata
            key = f"payload-{len(self.storage) + len(entries)}"
            serialized = payload.SerializeToString()
            entries.append((key, serialized))
            snapshot = Payload()
            snapshot.ParseFromString(serialized)
            self.stored_payloads.append(snapshot)
        self.storage.update(entries)
        await asyncio.sleep(0)
        return [StorageDriverClaim(claim_data={"key": key}) for key, _ in entries]

    async def retrieve(
        self,
        context: StorageDriverRetrieveContext,
        claims: Sequence[StorageDriverClaim],
    ) -> list[Payload]:
        _ = context
        payloads: list[Payload] = []
        for claim in claims:
            serialized = self.storage.get(claim.claim_data["key"])
            if serialized is None:
                raise ApplicationError("stored payload not found", non_retryable=True)
            payload = Payload()
            payload.ParseFromString(serialized)
            payloads.append(payload)
        return payloads


storage_driver = RecordingStorageDriver()


@workflow.defn
class TargetWorkflow:
    def __init__(self) -> None:
        self.signals: list[str] = []

    @workflow.run
    async def run(self, value: str) -> list[str]:
        await workflow.wait_condition(lambda: len(self.signals) == 2)
        return [f"started: {value}", *self.signals]

    @workflow.signal
    def add(self, value: str) -> None:
        self.signals.append(f"signal: {value}")


@workflow.defn
class CallerWorkflow:
    @workflow.run
    async def run(self, target_id: str, task_queue: str) -> str:
        await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            "start-value",
            id=target_id,
            task_queue=task_queue,
            signal=TargetWorkflow.add,
            signal_args="signal-one",
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            "unused-start-value",
            id=target_id,
            task_queue=task_queue,
            signal=TargetWorkflow.add,
            signal_args="signal-two",
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        return target_id


@workflow.defn
class ContextTargetWorkflow:
    def __init__(self) -> None:
        self.signals: list[ContextValue] = []

    @workflow.run
    async def run(self, value: ContextValue) -> list[str]:
        await workflow.wait_condition(lambda: len(self.signals) == 2)
        return [value.label, *(signal.label for signal in self.signals)]

    @workflow.signal
    def add(self, value: ContextValue) -> None:
        self.signals.append(value)


@workflow.defn
class ContextCallerWorkflow:
    @workflow.run
    async def run(self, target_id: str, task_queue: str) -> str:
        await workflow.signal_with_start_workflow(
            ContextTargetWorkflow.run,
            ContextValue("start-value"),
            id=target_id,
            task_queue=task_queue,
            signal=ContextTargetWorkflow.add,
            signal_args=ContextValue("signal-one"),
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        await workflow.signal_with_start_workflow(
            ContextTargetWorkflow.run,
            ContextValue("unused-start-value"),
            id=target_id,
            task_queue=task_queue,
            signal=ContextTargetWorkflow.add,
            signal_args=ContextValue("signal-two"),
            id_conflict_policy=WorkflowIDConflictPolicy.USE_EXISTING,
        )
        return target_id


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.client.start_workflow(
        CallerWorkflow.run,
        args=[f"{runner.feature.rel_dir}-target", runner.task_queue],
        id=f"{runner.feature.rel_dir}-caller",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    target_id = await handle.result()
    target = runner.client.get_workflow_handle(target_id)
    assert await target.result() == [
        "started: start-value",
        "signal: signal-one",
        "signal: signal-two",
    ]

    serialization_records.clear()
    storage_driver.storage.clear()
    storage_driver.stored_payloads.clear()
    target_id = f"{runner.feature.rel_dir}-context-target"
    context_handle = await runner.client.start_workflow(
        ContextCallerWorkflow.run,
        args=[target_id, runner.task_queue],
        id=f"{runner.feature.rel_dir}-context-caller",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    assert await context_handle.result() == target_id
    target = runner.client.get_workflow_handle(target_id)
    assert await target.result() == ["start-value", "signal-one", "signal-two"]

    expected_labels = {"start-value", "signal-one", "signal-two"}
    for operation in (
        "converter-encode",
        "converter-decode",
        "codec-encode",
        "codec-decode",
    ):
        records = [record for record in serialization_records if record[0] == operation]
        assert expected_labels.issubset({record[1] for record in records}), records
        assert all(record[2] == target_id for record in records)

    stored_context_values = {
        json.loads(payload.data)["label"]
        for payload in storage_driver.stored_payloads
        if CONTEXT_MARKER in payload.metadata
    }
    assert expected_labels.issubset(stored_context_values)
    assert all(
        payload.metadata[CONTEXT_MARKER] == target_id.encode()
        for payload in storage_driver.stored_payloads
        if CONTEXT_MARKER in payload.metadata
    )
    assert all(
        payload.metadata.get(CODEC_MARKER) == target_id.encode()
        for payload in storage_driver.stored_payloads
        if CONTEXT_MARKER in payload.metadata
    )


register_feature(
    workflows=[
        CallerWorkflow,
        ContextCallerWorkflow,
        ContextTargetWorkflow,
        TargetWorkflow,
    ],
    start=start,
    check_result=check_result,
    data_converter=DataConverter(
        payload_converter_class=ContextPayloadConverterSet,
        payload_codec=ContextPayloadCodec(serialization_records),
        external_storage=ExternalStorage(
            drivers=[storage_driver],
            payload_size_threshold=1,
        ),
    ),
)
