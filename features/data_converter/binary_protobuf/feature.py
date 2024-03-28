import dataclasses

from temporalio import workflow
from temporalio.api.common.v1 import DataBlob
from temporalio.client import WorkflowHandle
from temporalio.converter import (
    BinaryNullPayloadConverter,
    BinaryProtoPayloadConverter,
    CompositePayloadConverter,
    DataConverter,
)

from harness.python.feature import (
    Runner,
    get_workflow_argument_payload,
    get_workflow_result_payload,
    register_feature,
)

EXPECTED_RESULT = DataBlob(data=bytes.fromhex("deadbeef"))


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
    assert encoding == "binary/protobuf"

    message_type = payload.metadata["messageType"].decode("utf-8")
    assert message_type == "temporal.api.common.v1.DataBlob"

    result_in_history = DataBlob()
    result_in_history.ParseFromString(payload.data)
    assert result == result_in_history

    payload_arg = await get_workflow_argument_payload(handle)
    assert payload == payload_arg


class DefaultBinProtoPayloadConverter(CompositePayloadConverter):
    def __init__(self) -> None:
        super().__init__(
            # Disable ByteSlice, ProtoJSON, and JSON converters
            BinaryNullPayloadConverter(),
            BinaryProtoPayloadConverter(),
        )


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start_options={"arg": EXPECTED_RESULT},
    data_converter=dataclasses.replace(
        DataConverter.default, payload_converter_class=DefaultBinProtoPayloadConverter
    ),
)
