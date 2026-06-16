import uuid
from datetime import timedelta
from typing import Optional

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.common import WorkflowIDConflictPolicy

from harness.python.feature import Runner, register_feature

SIGNAL_VALUE = "test-signal-value"

_setup: dict = {}


@workflow.defn
class TargetWorkflow:
    def __init__(self) -> None:
        self._signal_value: Optional[str] = None

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._signal_value is not None)
        assert self._signal_value is not None
        return self._signal_value

    @workflow.signal
    def my_signal(self, value: str) -> None:
        self._signal_value = value


@workflow.defn
class CallerWorkflow:
    @workflow.run
    async def run(self, target_id: str) -> str:
        # WORKFLOW_ID_CONFLICT_POLICY_FAIL is rejected by the server when
        # scheduling the Nexus operation (signal-with-required-start is not a
        # supported operation). This surfaces as a workflow task failure rather
        # than a catchable error, so this run never completes; the client asserts
        # on the workflow task failure in history.
        handle = await workflow.signal_with_start_workflow(
            TargetWorkflow.run,
            id=target_id,
            task_queue=workflow.info().task_queue,
            signal=TargetWorkflow.my_signal,
            signal_args=SIGNAL_VALUE,
            id_conflict_policy=WorkflowIDConflictPolicy.FAIL,
        )
        return handle.run_id


async def start(runner: Runner) -> WorkflowHandle:
    target_id = f"signalwithstart-id-conflict-fail-target-{uuid.uuid4()}"
    _setup["target_id"] = target_id

    # Start a running target to mirror the server test setup.
    target = await runner.client.start_workflow(
        TargetWorkflow.run,
        id=target_id,
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )
    _setup["target_run_id"] = target.result_run_id

    return await runner.client.start_workflow(
        CallerWorkflow.run,
        target_id,
        id=f"signalwithstart-id-conflict-fail-caller-{uuid.uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # The caller never completes; wait for the workflow task failure that rejects
    # the unsupported conflict policy.
    event = await runner.wait_for_event(
        handle,
        lambda e: e.event_type == EventType.EVENT_TYPE_WORKFLOW_TASK_FAILED,
        timeout=30.0,
    )
    message = event.workflow_task_failed_event_attributes.failure.message
    assert (
        "not supported" in message.lower()
    ), f"expected 'not supported' rejection, got: {message}"

    # Cleanup the caller (stuck failing its workflow task) and the target.
    for wf_id, run_id in (
        (handle.id, None),
        (_setup["target_id"], _setup["target_run_id"]),
    ):
        try:
            await runner.client.get_workflow_handle(wf_id, run_id=run_id).terminate(
                "cleanup"
            )
        except Exception:
            pass


register_feature(
    workflows=[CallerWorkflow, TargetWorkflow],
    start=start,
    check_result=check_result,
)
