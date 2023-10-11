from datetime import timedelta

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

WORKFLOW_INITIAL_STATE = ""
UPDATE_ARG = "update-arg"
BAD_UPDATE_ARG = "reject-me"
UPDATE_RESULT = "update-result"


@workflow.defn(name="Workflow")
class Workflow:
    """
    A workflow with a signal and signal validator.
    If accepted, the signal makes a change to workflow state.
    The workflow does not terminate until such a change occurs.
    """

    def __init__(self) -> None:
        self._state = WORKFLOW_INITIAL_STATE

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._state != WORKFLOW_INITIAL_STATE)
        return self._state

    @workflow.update
    async def my_update(self, arg: str) -> str:
        self._state = arg
        return UPDATE_RESULT

    @my_update.validator
    def my_validate(self, arg: str):
        if arg == BAD_UPDATE_ARG:
            raise ValueError("Invalid Update argument")


async def checker(_: Runner, handle: WorkflowHandle):
    # TODO: check server supports update
    try:
        await handle.execute_update("my_update", BAD_UPDATE_ARG)
    except:
        pass
    else:
        assert False, "Expected Update to be rejected due to validation failure"

    update_result = await handle.execute_update("my_update", UPDATE_ARG)
    assert update_result == UPDATE_RESULT
    result = await handle.result()
    assert result == UPDATE_ARG


register_feature(workflows=[Workflow], check_result=checker)
