from typing import Optional

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

SIGNAL_DATA = "Signaled!"


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self._result: Optional[str] = None

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self._result is not None)
        return str(self._result)

    @workflow.signal
    async def external_signal(self, s: str) -> None:
        self._result = s


async def start_then_signal(runner: Runner) -> WorkflowHandle:
    handle = await runner.start_single_parameterless_workflow()
    await handle.signal(Workflow.external_signal, SIGNAL_DATA)
    return handle


register_feature(
    workflows=[Workflow],
    expect_run_result=SIGNAL_DATA,
    start=start_then_signal,
)
