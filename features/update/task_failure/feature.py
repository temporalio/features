from datetime import timedelta

from temporalio import workflow
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
        # controlled number of times.
        global task_fails_counter
        if task_fails_counter < 2:
            task_fails_counter += 1
            raise RuntimeError("I'll fail task")
        else:
            raise ApplicationError("I'll fail update")

    @workflow.update
    async def throw_or_done(self, do_throw: bool):
        self.am_done = True

    @throw_or_done.validator
    def the_validator(self, do_throw: bool):
        if do_throw:
            raise RuntimeError("This will fail validation, not task")


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    try:
        await handle.execute_update(Workflow.do_update)
        raise RuntimeError("Should throw")
    except WorkflowUpdateFailedError:
        pass

    try:
        await handle.execute_update(Workflow.throw_or_done, True)
        raise RuntimeError("Should throw")
    except WorkflowUpdateFailedError:
        pass

    await handle.execute_update(Workflow.throw_or_done, False)
    await handle.result()
    global task_fails_counter
    assert task_fails_counter == 2


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
    worker_config=WorkerConfig(workflow_runner=UnsandboxedWorkflowRunner()),
    # A shorter task timeout to make the task retry happen faster
    start_options={"task_timeout": timedelta(seconds=3)},
)
