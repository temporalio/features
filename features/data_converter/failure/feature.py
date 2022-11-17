import json
from datetime import timedelta

from google.protobuf.json_format import Parse
from temporalio import activity, workflow
from temporalio.api.common.v1 import Payload
from temporalio.api.enums.v1 import EventType
from temporalio.api.failure.v1 import Failure
from temporalio.client import WorkflowHandle
from temporalio.common import RetryPolicy
from temporalio.converter import (
    DataConverter,
    DefaultFailureConverterWithEncodedAttributes,
)
from temporalio.exceptions import ActivityError, ApplicationError, CancelledError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    """
    run a workflow that calls an activity that fails
    """

    @workflow.run
    async def run(self) -> None:
        # Start workflow
        handle = workflow.start_activity(
            failure_activity,
            schedule_to_close_timeout=timedelta(minutes=1),
            heartbeat_timeout=timedelta(seconds=5),
            # Disable retry
            retry_policy=RetryPolicy(maximum_attempts=1),
        )

        try:
            await handle
            raise ApplicationError("Activity should have thrown exception")
        except ActivityError as err:
            if not isinstance(err.cause, ApplicationError):
                raise ApplicationError("Expected application error") from err


@activity.defn
async def failure_activity() -> None:
    raise ApplicationError("main error") from ApplicationError("cause error")


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    await handle.result()

    # get result payload of ActivityTaskFailedEventAttributes event from workflow history
    event = await anext(
        e
        async for e in handle.fetch_history_events()
        if e.event_type == EventType.EVENT_TYPE_ACTIVITY_TASK_FAILED
    )
    failure = event.activity_task_failed_event_attributes.failure
    check_failure(failure, "main error")
    check_failure(failure.cause, "cause error")


def check_failure(failure: Failure, message: str):
    assert failure.message == "Encoded failure"
    assert failure.stack_trace == ""
    assert "json/plain" == failure.encoded_attributes.metadata["encoding"].decode(
        "utf-8"
    )
    data = json.loads(failure.encoded_attributes.data.decode("utf-8"))
    assert message == data["message"]
    assert "stack_trace" in data


register_feature(
    workflows=[Workflow],
    activities=[failure_activity],
    check_result=check_result,
    data_converter=DataConverter(
        failure_converter_class=DefaultFailureConverterWithEncodedAttributes
    ),
)
