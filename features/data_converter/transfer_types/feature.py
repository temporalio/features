from __future__ import annotations

import logging
import uuid
from dataclasses import dataclass
from datetime import timedelta
from typing import Generic, TypeVar, cast, get_args

import nexusrpc
from temporalio import activity, workflow
from temporalio.api.common.v1 import Payload, WorkflowExecution
from temporalio.api.enums.v1 import EventType
from temporalio.client import RPCError, RPCStatusCode, WorkflowHandle
from temporalio.common import RetryPolicy
from temporalio.converter import (
    JSONProtoPayloadConverter,
    TransferTypeConverter,
    transfer_type_convertible,
)
from temporalio.exceptions import ActivityError, ApplicationError

from harness.python.feature import Runner, register_feature

logger = logging.getLogger(__name__)
TRANSFERRED_MARKER = "created-from-transfer-type"


class NonGenericValueConverter(
    TransferTypeConverter["NonGenericValue", WorkflowExecution]
):
    transfer_type = WorkflowExecution

    def to_transfer_type(self, value: NonGenericValue) -> WorkflowExecution:
        return WorkflowExecution(workflow_id=value.value, run_id="non-generic")

    def from_transfer_type(
        self, value: WorkflowExecution, type_hint: type[NonGenericValue]
    ) -> NonGenericValue:
        assert type_hint is NonGenericValue
        assert value.run_id == "non-generic"
        return NonGenericValue(value=value.workflow_id, extra=TRANSFERRED_MARKER)


@transfer_type_convertible(NonGenericValueConverter)
@dataclass
class NonGenericValue:
    value: str
    extra: str


T = TypeVar("T")


@dataclass
class Box(Generic[T]):
    value: T
    extra: str


class BoxConverter(TransferTypeConverter[Box[T], WorkflowExecution]):
    transfer_type = WorkflowExecution

    def to_transfer_type(self, value: Box[T]) -> WorkflowExecution:
        return WorkflowExecution(workflow_id=str(value.value), run_id="box")

    def from_transfer_type(
        self, value: WorkflowExecution, type_hint: type[Box[T]]
    ) -> Box[T]:
        assert value.run_id == "box"
        type_args = get_args(type_hint)
        item_type = type_args[0] if type_args else str
        item: str | int = value.workflow_id
        if item_type is int:
            item = int(item)
        return Box(value=cast(T, item), extra=TRANSFERRED_MARKER)


transfer_type_convertible(BoxConverter)(Box)


class ConvertedBaseConverter(TransferTypeConverter["ConvertedBase", WorkflowExecution]):
    transfer_type = WorkflowExecution

    def to_transfer_type(self, value: ConvertedBase) -> WorkflowExecution:
        return WorkflowExecution(workflow_id=value.value, run_id="converted-base")

    def from_transfer_type(
        self, value: WorkflowExecution, type_hint: type[ConvertedBase]
    ) -> ConvertedBase:
        assert value.run_id == "converted-base"
        return ConvertedBase(value=value.workflow_id, extra=TRANSFERRED_MARKER)


@transfer_type_convertible(ConvertedBaseConverter)
@dataclass
class ConvertedBase:
    value: str
    extra: str


@dataclass
class DerivedFromConvertedBase(ConvertedBase):
    derived_extra: str


@dataclass
class PlainBase:
    value: str
    extra: str


class ConvertedDerivedConverter(
    TransferTypeConverter["ConvertedDerived", WorkflowExecution]
):
    transfer_type = WorkflowExecution

    def to_transfer_type(self, value: ConvertedDerived) -> WorkflowExecution:
        return WorkflowExecution(workflow_id=value.value, run_id="converted-derived")

    def from_transfer_type(
        self, value: WorkflowExecution, type_hint: type[ConvertedDerived]
    ) -> ConvertedDerived:
        assert type_hint is ConvertedDerived
        assert value.run_id == "converted-derived"
        return ConvertedDerived(
            value=value.workflow_id,
            extra=TRANSFERRED_MARKER,
            derived_extra=TRANSFERRED_MARKER,
        )


@transfer_type_convertible(ConvertedDerivedConverter)
@dataclass
class ConvertedDerived(PlainBase):
    derived_extra: str


@dataclass
class PlainValue:
    value: str
    extra: str
    nested: NonGenericValue


class TransferConversionError(RuntimeError):
    pass


class ThrowingValueConverter(TransferTypeConverter["ThrowingValue", WorkflowExecution]):
    transfer_type = WorkflowExecution

    def to_transfer_type(self, value: ThrowingValue) -> WorkflowExecution:
        raise TransferConversionError(value.value)

    def from_transfer_type(
        self, value: WorkflowExecution, type_hint: type[ThrowingValue]
    ) -> ThrowingValue:
        raise TransferConversionError(value.workflow_id)


@transfer_type_convertible(ThrowingValueConverter)
@dataclass
class ThrowingValue:
    value: str


@activity.defn
async def fail_with_transfer_detail() -> None:
    raise ApplicationError(
        "intentional transfer detail failure",
        NonGenericValue("failure-detail", "must-not-be-serialized"),
        non_retryable=True,
    )


@activity.defn
async def standalone_transfer_activity(value: NonGenericValue) -> NonGenericValue:
    assert value == NonGenericValue("activity-input", TRANSFERRED_MARKER)
    return NonGenericValue("activity-result", "must-not-be-serialized")


@nexusrpc.service
class TransferService:
    transfer: nexusrpc.Operation[NonGenericValue, NonGenericValue]


@nexusrpc.handler.service_handler(service=TransferService)
class TransferServiceHandler:
    @nexusrpc.handler.sync_operation
    async def transfer(
        self, ctx: nexusrpc.handler.StartOperationContext, value: NonGenericValue
    ) -> NonGenericValue:
        assert value == NonGenericValue("nexus-input", TRANSFERRED_MARKER)
        return NonGenericValue("nexus-result", "must-not-be-serialized")


@workflow.defn
class Workflow:
    @workflow.run
    async def run(
        self,
        non_generic: NonGenericValue,
        box: Box[int],
        converted_base: ConvertedBase,
        unconverted_derived: DerivedFromConvertedBase,
        plain: PlainValue,
        plain_base: PlainBase,
        converted_derived: ConvertedDerived,
    ) -> NonGenericValue:
        failures: list[str] = []

        def check(condition: bool, name: str) -> None:
            if not condition:
                failures.append(name)

        check(
            non_generic == NonGenericValue("non-generic", TRANSFERRED_MARKER),
            "non-generic",
        )
        check(box == Box(123, TRANSFERRED_MARKER), "generic")
        check(type(converted_base) is ConvertedBase, "base-type")
        check(
            converted_base == ConvertedBase("converted-base", TRANSFERRED_MARKER),
            "base-converter",
        )
        check(
            type(unconverted_derived) is DerivedFromConvertedBase,
            "exact-declaration-type",
        )
        check(
            unconverted_derived
            == DerivedFromConvertedBase(
                "unconverted-derived", "plain-extra", "derived-extra"
            ),
            "exact-declaration-value",
        )
        check(type(plain_base) is PlainBase, "declared-plain-base-type")
        check(
            plain_base == PlainBase("plain-base", "plain-extra"),
            "declared-plain-base-value",
        )
        check(
            converted_derived
            == ConvertedDerived(
                "converted-derived", TRANSFERRED_MARKER, TRANSFERRED_MARKER
            ),
            "converted-derived",
        )
        check(
            plain
            == PlainValue(
                "plain",
                "plain-extra",
                NonGenericValue("nested", "nested-extra"),
            ),
            "top-level-only",
        )

        try:
            await workflow.execute_activity(
                fail_with_transfer_detail,
                start_to_close_timeout=timedelta(seconds=10),
            )
            failures.append("failure-detail-missing")
        except ActivityError as err:
            check(isinstance(err.cause, ApplicationError), "failure-detail-error")
            details = (
                err.cause.details if isinstance(err.cause, ApplicationError) else ()
            )
            check(len(details) == 1, "failure-detail-count")
            detail = details[0] if details else None
            check(isinstance(detail, WorkflowExecution), "failure-detail-type")
            check(
                detail
                == WorkflowExecution(
                    workflow_id="failure-detail", run_id="non-generic"
                ),
                "failure-detail-value",
            )

        result_value = "workflow-result"
        if failures:
            result_value = "failed:" + ",".join(failures)
        return NonGenericValue(result_value, "must-not-be-serialized")


@workflow.defn
class ThrowingWorkflow:
    @workflow.run
    async def run(self, value: ThrowingValue) -> None:
        raise AssertionError("a converter failure must prevent workflow start")


async def exercise_standalone_activity(runner: Runner) -> None:
    result = await runner.client.execute_activity(
        standalone_transfer_activity,
        NonGenericValue("activity-input", "client-extra"),
        id=f"{runner.feature.rel_dir}-activity-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        start_to_close_timeout=timedelta(seconds=10),
        retry_policy=RetryPolicy(maximum_attempts=1),
    )
    assert result == NonGenericValue("activity-result", TRANSFERRED_MARKER)


async def exercise_standalone_nexus(runner: Runner) -> None:
    if runner.nexus_endpoint is None:
        logger.info("Skipping Standalone Nexus check because no endpoint is available")
        return
    client = runner.client.create_nexus_client(
        service=TransferService,
        endpoint=runner.nexus_endpoint,
    )
    try:
        result = await client.execute_operation(
            TransferService.transfer,
            NonGenericValue("nexus-input", "client-extra"),
            id=f"{runner.feature.rel_dir}-nexus-{uuid.uuid4()}",
            schedule_to_close_timeout=timedelta(seconds=10),
        )
    except RPCError as err:
        if err.status == RPCStatusCode.UNIMPLEMENTED:
            logger.info(
                "Skipping Standalone Nexus check because the server does not support it"
            )
            return
        raise
    assert result == NonGenericValue("nexus-result", TRANSFERRED_MARKER)


async def start(runner: Runner) -> WorkflowHandle:
    failed_workflow_id = f"{runner.feature.rel_dir}-failing-{uuid.uuid4()}"
    try:
        await runner.client.start_workflow(
            ThrowingWorkflow.run,
            ThrowingValue("expected transfer conversion failure"),
            id=failed_workflow_id,
            task_queue=runner.task_queue,
            execution_timeout=timedelta(minutes=1),
        )
        raise AssertionError("workflow start should fail during transfer conversion")
    except TransferConversionError as err:
        assert str(err) == "expected transfer conversion failure"

    try:
        await runner.client.get_workflow_handle(failed_workflow_id).describe()
        raise AssertionError("converter failure should prevent the workflow RPC")
    except RPCError as err:
        assert err.status == RPCStatusCode.NOT_FOUND

    await exercise_standalone_activity(runner)
    await exercise_standalone_nexus(runner)

    return await runner.client.start_workflow(
        Workflow.run,
        args=[
            NonGenericValue("non-generic", "client-extra"),
            Box(123, "client-extra"),
            DerivedFromConvertedBase(
                "converted-base", "client-extra", "client-derived-extra"
            ),
            DerivedFromConvertedBase(
                "unconverted-derived", "plain-extra", "derived-extra"
            ),
            PlainValue(
                "plain",
                "plain-extra",
                NonGenericValue("nested", "nested-extra"),
            ),
            ConvertedDerived("plain-base", "plain-extra", "ignored-derived-extra"),
            ConvertedDerived(
                "converted-derived", "client-extra", "client-derived-extra"
            ),
        ],
        id=f"{runner.feature.rel_dir}-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    result = await handle.result()
    assert result == NonGenericValue("workflow-result", TRANSFERRED_MARKER), (
        result.value
    )

    events = [event async for event in handle.fetch_history_events()]
    started = next(
        event
        for event in events
        if event.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED
    )
    inputs = started.workflow_execution_started_event_attributes.input.payloads
    assert len(inputs) == 7
    assert_protobuf_payload(inputs[0], "non-generic")
    assert_protobuf_payload(inputs[1], "box")
    assert_protobuf_payload(inputs[2], "converted-base")
    assert_json_payload(inputs[3])
    assert_json_payload(inputs[4])
    assert_json_payload(inputs[5])
    assert_protobuf_payload(inputs[6], "converted-derived")

    activity_failed = next(
        event
        for event in events
        if event.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_FAILED
    )
    detail_payload = activity_failed.activity_task_failed_event_attributes.failure.application_failure_info.details.payloads[
        0
    ]
    assert_protobuf_payload(detail_payload, "non-generic")

    completed = next(
        event
        for event in events
        if event.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED
    )
    assert_protobuf_payload(
        completed.workflow_execution_completed_event_attributes.result.payloads[0],
        "non-generic",
    )


def assert_protobuf_payload(payload: Payload, expected_run_id: str) -> None:
    assert payload.metadata["encoding"] == b"json/protobuf"
    assert (
        payload.metadata["messageType"] == b"temporal.api.common.v1.WorkflowExecution"
    )
    value = JSONProtoPayloadConverter().from_payload(payload, WorkflowExecution)
    assert isinstance(value, WorkflowExecution)
    assert value.run_id == expected_run_id


def assert_json_payload(payload: Payload) -> None:
    assert payload.metadata["encoding"] == b"json/plain"


register_feature(
    workflows=[Workflow, ThrowingWorkflow],
    activities=[fail_with_transfer_detail, standalone_transfer_activity],
    nexus_service_handlers=[TransferServiceHandler()],
    start=start,
    check_result=check_result,
)
