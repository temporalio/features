import os

from google.protobuf.json_format import Parse

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.api.common.v1 import Payload

from harness.python.feature import register_feature, Runner


@workflow.defn
class Workflow:
    """
    run a workflow that returns binary value `0xdeadbeef`
    """
    @workflow.run
    async def run(self) -> bytes:
        return bytes.fromhex("deadbeef")


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # verify client result is binary `0xdeadbeef`
    result = await handle.result()
    assert result == bytes.fromhex("deadbeef")

    # get result payload of WorkflowExecutionCompleted event from workflow history
    history = await runner.get_history_events(handle)
    e = next(e for e in history if e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED)
    payload = e.workflow_execution_completed_event_attributes.result.payloads[0]

    # load JSON payload from `./payload.json` and compare it to JSON representation of result payload
    with open(os.path.join(os.path.dirname(__file__), 'payload.json'), encoding='ascii') as f:
        expected_payload = Parse(f.read(), Payload())
    assert payload == expected_payload


register_feature(
    workflows=[Workflow],
    check_result=check_result,
)
