import os
from datetime import timedelta
from typing import Optional

from google.protobuf.json_format import Parse
from temporalio import activity, workflow
from temporalio.api.common.v1 import Payload
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@activity.defn
async def null_activity(input: Optional[str]) -> None:
    # check the null input is serialized correctly
    if input != None:
        raise ApplicationError("Activity input should be None", non_retryable=True)


@workflow.defn
class Workflow:
    """
    run a workflow that calls an activity with a None parameter.
    """

    @workflow.run
    async def run(self) -> None:
        handle = workflow.start_activity(
            null_activity,
            None,
            start_to_close_timeout=timedelta(minutes=1),
        )
        await handle


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # verify client result is None
    result = await handle.result()
    assert result == None

    # get result payload of ActivityTaskScheduled event from workflow history
    event = await anext(
        e
        async for e in handle.fetch_history_events()
        if e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED
    )
    payload = event.activity_task_scheduled_event_attributes.input.payloads[0]

    # load JSON payload from `./payload.json` and compare it to JSON representation of result payload
    with open(
        os.path.join(os.path.dirname(runner.feature.file), "payload.json"),
        encoding="ascii",
    ) as f:
        expected_payload = Parse(f.read(), Payload())

    assert payload == expected_payload


register_feature(
    workflows=[Workflow],
    activities=[null_activity],
    check_result=check_result,
)
