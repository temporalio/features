import json
from typing import Dict

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import (
    Runner,
    get_workflow_argument_payload,
    get_workflow_result_payload,
    register_feature,
)

Result = Dict[str, bool]

EXPECTED_RESULT: Result = {"spec": True}

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
    assert encoding == "json/plain"

    result_in_history = json.loads(payload.data)
    assert result == result_in_history

    payload_arg = await get_workflow_argument_payload(handle)
    assert payload == payload_arg


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start_options={"arg": EXPECTED_RESULT},
)
