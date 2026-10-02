"""Context aware converters shared by the serialization_context features.

The codec stamps the signature of its serialization context onto every payload it
encodes and refuses to decode a payload encoded under a different context, so any
asymmetry between the encoding and the decoding side fails the feature wherever
it happens. Each feature additionally asserts the exact signature recorded in
history.
"""

from __future__ import annotations

import dataclasses
from typing import Any, List, Optional, Sequence

from temporalio.api.common.v1 import Payload
from temporalio.api.failure.v1 import Failure
from temporalio.client import WorkflowHandle
from temporalio.converter import (
    ActivitySerializationContext,
    DataConverter,
    DefaultFailureConverter,
    PayloadCodec,
    PayloadConverter,
    SerializationContext,
    WithSerializationContext,
    WorkflowSerializationContext,
)

METADATA_KEY = "ctx-signature"
NO_CONTEXT = "none"

observed_signatures: set[str] = set()
"""Every signature the codec has been asked to encode or decode with.

Worker and client share a process here, so this is how a feature asserts on a
context whose payload never shows up in history.
"""


def workflow_signature(namespace: str, workflow_id: str) -> str:
    return f"wf|{namespace}|{workflow_id}"


def activity_signature(
    namespace: Optional[str],
    workflow_id: Optional[str],
    workflow_type: Optional[str],
    activity_type: Optional[str],
    activity_id: Optional[str],
    activity_task_queue: Optional[str],
    is_local: bool,
) -> str:
    return (
        f"act|{namespace}|{workflow_id}|{workflow_type}|{activity_type}"
        f"|{activity_id}|{activity_task_queue}|{is_local}"
    )


def signature(context: Optional[SerializationContext]) -> str:
    if isinstance(context, WorkflowSerializationContext):
        return workflow_signature(context.namespace, context.workflow_id)
    if isinstance(context, ActivitySerializationContext):
        return activity_signature(
            context.namespace,
            context.workflow_id,
            context.workflow_type,
            context.activity_type,
            context.activity_id,
            context.activity_task_queue,
            context.is_local,
        )
    return NO_CONTEXT


class SigningCodec(PayloadCodec, WithSerializationContext):
    def __init__(self, sig: str = NO_CONTEXT) -> None:
        super().__init__()
        self.signature = sig

    def with_context(self, context: SerializationContext) -> SigningCodec:
        return SigningCodec(signature(context))

    async def encode(self, payloads: Sequence[Payload]) -> List[Payload]:
        observed_signatures.add(self.signature)
        result: List[Payload] = []
        for p in payloads:
            clone = Payload()
            clone.CopyFrom(p)
            clone.metadata[METADATA_KEY] = self.signature.encode()
            result.append(clone)
        return result

    async def decode(self, payloads: Sequence[Payload]) -> List[Payload]:
        observed_signatures.add(self.signature)
        result: List[Payload] = []
        for p in payloads:
            encoded = signature_of(p)
            if encoded != self.signature:
                raise ValueError(
                    f"serialization context mismatch: payload encoded as {encoded!r}, "
                    f"decoded as {self.signature!r}"
                )
            clone = Payload()
            clone.CopyFrom(p)
            del clone.metadata[METADATA_KEY]
            result.append(clone)
        return result


class SigningFailureConverter(DefaultFailureConverter, WithSerializationContext):
    def __init__(self, sig: str = NO_CONTEXT) -> None:
        super().__init__()
        self.signature = sig

    def with_context(self, context: SerializationContext) -> SigningFailureConverter:
        return SigningFailureConverter(signature(context))

    def to_failure(
        self,
        exception: BaseException,
        payload_converter: PayloadConverter,
        failure: Failure,
    ) -> None:
        super().to_failure(exception, payload_converter, failure)
        # A failure that already travelled the wire is copied as-is by the
        # default converter, and its source must not be overwritten.
        if not failure.source:
            failure.source = self.signature


def data_converter() -> DataConverter:
    return dataclasses.replace(
        DataConverter.default,
        payload_codec=SigningCodec(),
        failure_converter_class=SigningFailureConverter,
    )


def signature_of(payload: Payload) -> str:
    return payload.metadata.get(METADATA_KEY, b"").decode()


def first_signature(payloads: Any) -> str:
    return signature_of(payloads.payloads[0])


async def events(handle: WorkflowHandle) -> List[Any]:
    return [e async for e in handle.fetch_history_events()]


def find_event(events: List[Any], name: str, predicate) -> Any:
    for event in events:
        if predicate(event):
            return event
    raise AssertionError(f"no {name} event in history")
