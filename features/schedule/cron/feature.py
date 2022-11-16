import asyncio
import logging

from temporalio import workflow
from temporalio.client import WorkflowExecutionStatus, WorkflowHandle

from harness.python.feature import Runner, register_feature

logger = logging.getLogger(__name__)


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        assert workflow.info().cron_schedule == "@every 2s"


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    try:
        # Try 10 times (waiting 1s before each) to get at least 2 completions
        for _ in range(10):
            await asyncio.sleep(1)
            completed = 0
            async for exec in runner.client.list_workflows(
                f"WorkflowId = '{handle.id}'"
            ):
                if exec.status == WorkflowExecutionStatus.COMPLETED:
                    completed += 1
                elif exec.status != WorkflowExecutionStatus.RUNNING:
                    raise RuntimeError("Not running")
            if completed >= 2:
                return
        raise RuntimeError("Did not get at least 2 completed")
    finally:
        # Terminate on complete
        try:
            await handle.terminate("feature complete")
        except:
            logger.exception("Failed terminating workflow")


register_feature(
    workflows=[Workflow],
    start_options={"cron_schedule": "@every 2s"},
    check_result=check_result,
)
