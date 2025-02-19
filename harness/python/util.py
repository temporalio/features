import asyncio
import time
from datetime import timedelta
from typing import Awaitable, Callable, TypeVar

from temporalio.api.common.v1 import WorkflowExecution
from temporalio.api.update.v1 import UpdateRef
from temporalio.api.workflowservice.v1 import PollWorkflowExecutionUpdateRequest
from temporalio.client import Client, WorkflowHandle
from temporalio.service import RPCError, RPCStatusCode
from temporalio.workflow import UpdateMethodMultiParam

# The update utilities below are copied from
# https://github.com/temporalio/sdk-python/blob/main/tests/helpers/__init__.py


async def admitted_update_task(
    client: Client,
    handle: WorkflowHandle,
    update_method: UpdateMethodMultiParam,
    id: str,
    **kwargs,
) -> asyncio.Task:
    """
    Return an asyncio.Task for an update after waiting for it to be admitted.
    """
    update_task = asyncio.create_task(
        handle.execute_update(update_method, id=id, **kwargs)
    )
    await assert_eq_eventually(
        True,
        lambda: workflow_update_has_been_admitted(client, handle.id, id),
    )
    return update_task


async def workflow_update_has_been_admitted(
    client: Client, workflow_id: str, update_id: str
) -> bool:
    try:
        await client.workflow_service.poll_workflow_execution_update(
            PollWorkflowExecutionUpdateRequest(
                namespace=client.namespace,
                update_ref=UpdateRef(
                    workflow_execution=WorkflowExecution(workflow_id=workflow_id),
                    update_id=update_id,
                ),
            )
        )
        return True
    except RPCError as err:
        if err.status != RPCStatusCode.NOT_FOUND:
            raise
        return False


T = TypeVar("T")


async def assert_eq_eventually(
    expected: T,
    fn: Callable[[], Awaitable[T]],
    *,
    timeout: timedelta = timedelta(seconds=10),
    interval: timedelta = timedelta(milliseconds=200),
) -> None:
    start_sec = time.monotonic()
    last_value = None
    while timedelta(seconds=time.monotonic() - start_sec) < timeout:
        last_value = await fn()
        if expected == last_value:
            return
        await asyncio.sleep(interval.total_seconds())
    assert expected == last_value, (
        f"timed out waiting for equal, asserted against last value of {last_value}"
    )
