from datetime import timedelta
from typing import Optional
from uuid import uuid4

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

UNBLOCK_MESSAGE = "unblock"


@workflow.defn
class Workflow:
    """
    A workflow that starts a child workflow, unblocks it, and returns the result of the child workflow.
    """

    @workflow.run
    async def run(self) -> str:
        child_handle = await workflow.start_child_workflow(ChildWorkflow.run)
        await child_handle.signal(ChildWorkflow.unblock, UNBLOCK_MESSAGE)
        return await child_handle


@workflow.defn
class ChildWorkflow:
    """
    A workflow that waits for a signal and returns the data received.
    """

    def __init__(self) -> None:
        self._unblock_message: Optional[str] = None

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._unblock_message is not None)
        assert self._unblock_message is not None
        return self._unblock_message

    @workflow.signal
    def unblock(self, message: Optional[str]) -> None:
        self._unblock_message = message


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.start_parameterless_workflow(Workflow)


register_feature(
    workflows=[Workflow, ChildWorkflow],
    expect_run_result=UNBLOCK_MESSAGE,
    start=start,
)
