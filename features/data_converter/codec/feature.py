import base64
import dataclasses
import json
from typing import Dict, List, Sequence

from temporalio import workflow
from temporalio.api.common.v1 import Payload
from temporalio.client import WorkflowHandle
from temporalio.converter import DataConverter, PayloadCodec

from harness.python.feature import (
    Runner,
    get_workflow_argument_payload,
    get_workflow_result_payload,
    register_feature,
)

Result = Dict[str, bool]

EXPECTED_RESULT: Result = {"spec": True}

CODEC_ENCODING = "my_encoding"

# An echo workflow
@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, res: Result) -> Result:
        return res


async def check_result(_: Runner, handle: WorkflowHandle) -> None:
    # verify client result is `{"spec": true}`
    result = await handle.result()
    assert result == EXPECTED_RESULT
    payload = await get_workflow_result_payload(handle)

    encoding = payload.metadata["encoding"].decode("utf-8")
    assert encoding == CODEC_ENCODING

    extractedData = base64.b64decode(payload.data)
    innerPayload = Payload()
    innerPayload.ParseFromString(extractedData)

    encoding = innerPayload.metadata["encoding"].decode("utf-8")
    assert encoding == "json/plain"

    result_in_history = json.loads(innerPayload.data)
    assert result == result_in_history

    payload_arg = await get_workflow_argument_payload(handle)
    assert payload == payload_arg


# Based on samples-python/encryption/codec.py
class Base64PayloadCodec(PayloadCodec):
    def __init__(self) -> None:
        super().__init__()

    async def encode(self, payloads: Sequence[Payload]) -> List[Payload]:
        return [
            Payload(
                metadata={
                    "encoding": b"my_encoding",
                },
                data=base64.b64encode(p.SerializeToString()),
            )
            for p in payloads
        ]

    async def decode(self, payloads: Sequence[Payload]) -> List[Payload]:
        ret: List[Payload] = []
        for p in payloads:
            if p.metadata.get("encoding", b"").decode() != CODEC_ENCODING:
                ret.append(p)
                continue
            ret.append(Payload.FromString(base64.b64decode(p.data)))
        return ret


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start_options={"arg": EXPECTED_RESULT},
    data_converter=dataclasses.replace(
        DataConverter.default,
        payload_codec=Base64PayloadCodec(),
    ),
)
