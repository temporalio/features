import asyncio
from datetime import timedelta

from temporalio import activity, workflow
from temporalio.client import WorkflowHandle, WorkflowUpdateFailedError
from temporalio.exceptions import ApplicationError
from temporalio.worker import UnsandboxedWorkflowRunner, WorkerConfig

from harness.python.feature import Runner, register_feature

task_fails_counter = 0


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self.am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_update(self):
        # Don't use global variables like this. We do here because we need to fail the task a
        # controlled number of times. Task failure is used here as a way to force a replay.
        global task_fails_counter
        if task_fails_counter == 0:
            task_fails_counter += 1
            raise RuntimeError("I'll fail task")
        self.am_done = True

    @do_update.validator
    def the_validator(self):
        # We will start rejecting things once we've failed the task, and hence are now replaying.
        # The fact that the workflow completes demonstrates that even though the validator would
        # "reject" on replay, it doesn't even run, since the update has already been accepted.
        global task_fails_counter
        if task_fails_counter > 1:
            raise ApplicationError("I would reject if I even ran :|")


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    await handle.execute_update(Workflow.do_update)
    await handle.result()
    global task_fails_counter
    assert task_fails_counter == 1


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
    worker_config=WorkerConfig(workflow_runner=UnsandboxedWorkflowRunner()),
)
