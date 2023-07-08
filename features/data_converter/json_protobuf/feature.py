from temporalio import workflow
from temporalio.api.common.v1 import DataBlob
from temporalio.client import WorkflowHandle
from temporalio.converter import JSONProtoPayloadConverter

from harness.python.feature import (
    Runner,
    get_workflow_argument_payload,
    get_workflow_result_payload,
    register_feature,
)

EXPECTED_RESULT = DataBlob(data=bytes.fromhex("deadbeef"))
JSONP_decoder = JSONProtoPayloadConverter()

# An echo workflow
@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, res: DataBlob) -> DataBlob:
        return res


async def check_result(_: Runner, handle: WorkflowHandle) -> None:
    # verify client result is DataBlob `0xdeadbeef`
    result = await handle.result()
    assert result == EXPECTED_RESULT
    payload = await get_workflow_result_payload(handle)

    encoding = payload.metadata["encoding"].decode("utf-8")
    assert encoding == "json/protobuf"

    message_type = payload.metadata["messageType"].decode("utf-8")
    assert message_type == "temporal.api.common.v1.DataBlob"

    result_in_history = JSONP_decoder.from_payload(payload)
    assert result == result_in_history

    payload_arg = await get_workflow_argument_payload(handle)
    assert payload == payload_arg


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start_options={"arg": EXPECTED_RESULT},
)
