import asyncio
from datetime import timedelta
from typing import Any

from temporalio import activity, workflow
from temporalio.client import (
    ClientConfig,
    Interceptor,
    OutboundInterceptor,
    StartWorkflowUpdateInput,
    WorkflowHandle,
    WorkflowUpdateHandle,
)

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    am_done = False

    @workflow.run
    async def run(self) -> str:
        await workflow.wait_condition(lambda: self.am_done)
        return "Hello, World!"

    @workflow.update
    async def do_update(self, arg: int):
        self.am_done = True
        return arg


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    assert 2 == await handle.execute_update(Workflow.do_update, 1)
    await handle.result()


class MyClientInterceptor(Interceptor):
    def intercept_client(self, next: OutboundInterceptor) -> OutboundInterceptor:
        return MyOutboundInterceptor(super().intercept_client(next))


class MyOutboundInterceptor(OutboundInterceptor):
    def __init__(self, next: OutboundInterceptor) -> None:
        super().__init__(next)

    async def start_workflow_update(
        self, input: StartWorkflowUpdateInput
    ) -> WorkflowUpdateHandle[Any]:
        if (
            input.update == "do_update"
        ):  # Need to ignore update testing if update is enabled
            input.args = [input.args[0] + 1]
        return await self.next.start_workflow_update(input)


register_feature(
    workflows=[Workflow],
    check_result=check_result,
    start=start,
    additional_client_config=ClientConfig(interceptors=[MyClientInterceptor()]),  # type: ignore
)
