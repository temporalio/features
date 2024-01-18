from datetime import timedelta

from temporalio import workflow
from temporalio.client import WorkflowHandle, WorkflowUpdateFailedError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    """
    A workflow with a signal and signal validator.
    If accepted, the signal makes a change to workflow state.
    The workflow does not terminate until such a change occurs.
    """

    def __init__(self) -> None:
        self._state = ""

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._state != "")
        return self._state

    @workflow.update
    async def my_update(self, arg: str) -> str:
        self._state = arg
        return "update-result"

    @my_update.validator
    def my_validate(self, arg: str):
        if arg == "bad-update-arg":
            raise ValueError("Invalid Update argument")


async def checker(runner: Runner, handle: WorkflowHandle):
    await runner.skip_if_update_unsupported()
    bad_update_handle = await handle.start_update(Workflow.my_update, "bad-update-arg")
    try:
        await bad_update_handle.result()
    except WorkflowUpdateFailedError:
        pass
    else:
        assert False, "Expected Update to be rejected due to validation failure"

    update_handle = await handle.start_update(Workflow.my_update, "update-arg")
    update_result = await update_handle.result()
    assert update_result == "update-result"
    result = await handle.result()
    assert result == "update-arg"


register_feature(workflows=[Workflow], check_result=checker)
