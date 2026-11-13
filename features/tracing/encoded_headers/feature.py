from __future__ import annotations

import dataclasses
import json
import uuid
from datetime import timedelta
from typing import Sequence

from temporalio import workflow

with workflow.unsafe.imports_passed_through():
    from opentelemetry import trace
    from opentelemetry.propagate import set_global_textmap
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.trace.propagation.tracecontext import (
        TraceContextTextMapPropagator,
    )
    from temporalio.api.common.v1 import Payload
    from temporalio.common import HeaderCodecBehavior
    from temporalio.contrib.opentelemetry import TracingInterceptor
    from temporalio.converter import DataConverter, PayloadCodec

    from harness.python.feature import Runner, register_feature


class WrappingCodec(PayloadCodec):
    async def encode(self, payloads: Sequence[Payload]) -> list[Payload]:
        result = []
        for payload in payloads:
            assert payload.metadata["encoding"] != b"binary/encrypted", (
                "Payload encoded twice"
            )
            result.append(
                Payload(
                    metadata={"encoding": b"binary/encrypted"},
                    data=bytes(b ^ 0x5A for b in payload.SerializeToString()),
                )
            )
        return result

    async def decode(self, payloads: Sequence[Payload]) -> list[Payload]:
        result = []
        for payload in payloads:
            assert payload.metadata["encoding"] == b"binary/encrypted"
            result.append(Payload.FromString(bytes(b ^ 0x5A for b in payload.data)))
        return result


@workflow.defn(name="workflow")
class Workflow:
    @workflow.run
    async def run(self, message: str) -> dict[str, str]:
        span = trace.get_current_span().get_span_context()
        assert span.is_valid, "No active workflow tracing span"
        return {"message": message, "traceId": f"{span.trace_id:032x}"}


async def start(runner: Runner):
    with trace.get_tracer("encoded-headers-feature").start_as_current_span(
        "python-caller"
    ) as parent:
        trace_id = f"{parent.get_span_context().trace_id:032x}"
        handle = await runner.client.start_workflow(
            "workflow",
            "hello",
            id=str(uuid.uuid4()),
            task_queue=runner.task_queue,
            execution_timeout=timedelta(seconds=15),
        )
        try:
            result = await handle.result()
        finally:
            history = await handle.fetch_history()
            for event in history.events:
                assert not event.HasField("workflow_task_failed_event_attributes"), (
                    event
                )
        header = history.events[
            0
        ].workflow_execution_started_event_attributes.header.fields["_tracer-data"]
        decoded = (await WrappingCodec().decode([header]))[0]
        carrier = json.loads(decoded.data)
        assert carrier["traceparent"].split("-")[1] == trace_id
        assert result == {"message": "hello", "traceId": trace_id}, result
        return handle


if not workflow.unsafe.in_sandbox():
    trace.set_tracer_provider(TracerProvider())
    set_global_textmap(TraceContextTextMapPropagator())
    register_feature(
        workflows=[Workflow],
        start=start,
        data_converter=dataclasses.replace(
            DataConverter.default, payload_codec=WrappingCodec()
        ),
        additional_client_config={
            "interceptors": [TracingInterceptor()],
            "header_codec_behavior": HeaderCodecBehavior.CODEC,
        },
    )
