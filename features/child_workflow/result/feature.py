from __future__ import annotations

from datetime import timedelta
from uuid import uuid4

from temporalio import workflow
from temporalio.client import WorkflowHandle

from harness.python.feature import Runner, register_feature

CHILD_WORKFLOW_INPUT = "Test"


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        return await workflow.execute_child_workflow(
            ChildWorkflow.run, CHILD_WORKFLOW_INPUT
        )


@workflow.defn
class ChildWorkflow:
    @workflow.run
    async def run(self, input: str) -> str:
        return input


async def start(runner: Runner) -> WorkflowHandle:
    return await runner.client.start_workflow(
        Workflow,
        id=f"{runner.feature.rel_dir}-{uuid4()}",
        task_queue=runner.task_queue,
        execution_timeout=timedelta(minutes=1),
    )


register_feature(
    workflows=[Workflow, ChildWorkflow],
    expect_run_result=CHILD_WORKFLOW_INPUT,
    start=start,
)
