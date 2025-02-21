"""
In this test the client code sends an update, guaranteeing that the workflow worker is
processing a workflow task (WFT) at the time that the update is admitted by the server. To
do this it must synchronize the workflow and client. This uses techniques that should
never be used in real workflows. The synchronization must be threading-based as opposed to
asyncio-based, since the point is to not allow the workflow to yield while it is waiting
for notification from the client. In order for the workflow and client to share the same
module namespace, we use UnsandboxedWorkflowRunner. But this means that the workflow and
client code execute in the same thread. Therefore we do the client's thread-blocking
synchronization calls in a new thread, via asyncio.to_thread, so that both client and
workflow can use thread-blocking waits on the shared threading.Event object.
"""

import asyncio
import threading
from datetime import timedelta
from uuid import uuid4

from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle
from temporalio.worker import UnsandboxedWorkflowRunner, WorkerConfig

from harness.python.feature import Runner, register_feature
from harness.python.util import admitted_update_task

# See docstring at top of file.
first_run_wft_is_in_progress = threading.Event()
update_has_been_admitted = threading.Event()


@workflow.defn
class Workflow:
    def __init__(self):
        self.received_update = False

    @workflow.run
    async def run(self) -> str:
        """
        Continue as new once, then return the current run ID.
        """
        if not first_run_wft_is_in_progress.is_set():
            # Note: you should usually never block the thread in workflow code.
            # See docstring at top of file.
            first_run_wft_is_in_progress.set()
            update_has_been_admitted.wait()

        info = workflow.info()
        if info.continued_run_id is not None:
            # The update is probably delivered in the first post-CAN WFT, in which case
            # the following wait_condition is not needed. However, correct behavior does
            # not require this to be true.
            await workflow.wait_condition(lambda: self.received_update)
            return info.run_id

        workflow.continue_as_new()

    @workflow.update
    async def update(self) -> str:
        """Update handler that returns the current run ID"""
        self.received_update = True
        return workflow.info().run_id


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.client.start_workflow(
        Workflow.run,
        id=str(uuid4()),
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    # See docstring at top of file.
    # Cause an update to be admitted while the first WFT is in progress
    await asyncio.to_thread(first_run_wft_is_in_progress.wait)
    # The workflow is now blocking its thread waiting for the update to be admitted
    update_task = await admitted_update_task(
        runner.client, handle, Workflow.update, "update-id"
    )
    # Unblock the workflow so that it responds to the WFT with a CAN command.
    update_has_been_admitted.set()
    # The workflow will now CAN. Wait for the update result
    update_run_id = await update_task

    # The update should have been handled on the post-CAN run.
    assert (
        handle.first_execution_run_id
        and update_run_id
        and update_run_id != handle.first_execution_run_id
    ), "Expected update to be handled on post-CAN run"

    update_event_types = {
        EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_ACCEPTED,
        EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_COMPLETED,
    }

    async def get_event_types(run_id: str) -> set[EventType.ValueType]:
        return {
            event.event_type
            for event in (
                await runner.client.get_workflow_handle(
                    handle.id, run_id=run_id
                ).fetch_history()
            ).events
        }

    assert not (
        update_event_types & await get_event_types(handle.first_execution_run_id)
    ), "Update should not appear in pre-CAN history"

    assert update_event_types <= await get_event_types(update_run_id), (
        "Update events should appear in post-CAN history"
    )


register_feature(
    workflows=[Workflow],
    start=start,
    check_result=check_result,
    # Disable sandbox in order to allow this test to "cheat" by sharing state between the
    # client and the workflow.
    worker_config=WorkerConfig(workflow_runner=UnsandboxedWorkflowRunner()),
)
