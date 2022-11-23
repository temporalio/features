import json
from datetime import timedelta

from google.protobuf.json_format import Parse
from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.api.failure.v1 import Failure
from temporalio.client import WorkflowHandle
from temporalio.converter import (
    DataConverter,
    DefaultFailureConverterWithEncodedAttributes,
)
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    """
    run a workflow that fails
    """

    @workflow.run
    async def run(self) -> None:
        raise ApplicationError("main error") from ApplicationError("cause error")


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    try:
        await handle.result()
        assert False
    except Exception:
        # get result payload of WorkflowExecutionFailed event from workflow history
        event = await anext(
            e
            async for e in handle.fetch_history_events()
            if e.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED
        )
        failure = event.workflow_execution_failed_event_attributes.failure
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
    check_result=check_result,
    data_converter=DataConverter(
        failure_converter_class=DefaultFailureConverterWithEncodedAttributes
    ),
)
