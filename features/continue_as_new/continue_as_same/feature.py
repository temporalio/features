from datetime import timedelta

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

INPUT_DATA = "InputData"
MEMO_KEY = "MemoKey"
MEMO_VALUE = "MemoValue"
WORKFLOW_ID = "TestID"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self, input: str) -> str:
        if workflow.info().continued_run_id is not None:
            return input
        workflow.continue_as_new(arg=input)


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.client.start_workflow(
        Workflow,
        id=WORKFLOW_ID,
        arg=INPUT_DATA,
        memo={
            MEMO_KEY: MEMO_VALUE,
        },
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    result = await handle.result()
    assert result == INPUT_DATA
    # Workflow ID does not change after continue as new
    assert handle.id == WORKFLOW_ID
    # Memos do not change after continue as new
    executionDescription = await handle.describe()
    testMemo = (await executionDescription.memo())[MEMO_KEY]
    assert testMemo == MEMO_VALUE


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
)
