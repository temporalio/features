from __future__ import annotations

import asyncio
import inspect
import logging
import uuid
from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path
from typing import (
    Any,
    Awaitable,
    Callable,
    Dict,
    List,
    Mapping,
    Optional,
    Type,
    TypedDict,
)

from temporalio import workflow
from temporalio.api.history.v1 import HistoryEvent
from temporalio.api.workflowservice.v1 import GetWorkflowExecutionHistoryRequest
from temporalio.client import Client, WorkflowFailureError, WorkflowHandle
from temporalio.exceptions import ActivityError, ApplicationError
from temporalio.worker import Worker

logger = logging.getLogger(__name__)

features: Dict[str, Feature] = {}


def register_feature(
    *,
    workflows: List[Type],
    activities: List[Callable] = [],
    expect_activity_error: Optional[str] = None,
    expect_run_result: Optional[Any] = None,
    file: Optional[str] = None,
    start: Optional[Callable[[Runner], Awaitable[WorkflowHandle]]] = None,
    check_result: Optional[Callable[[Runner, WorkflowHandle], Awaitable[None]]] = None,
) -> None:
    # No need to register in a sandbox
    if workflow.unsafe.in_sandbox():
        return

    if not file:
        file = inspect.stack()[1].filename
    # Split the file path to get the last two dirs if present
    parts = Path(file).parts
    if len(parts) <= 3:
        raise ValueError(f"Expected at least 3 path parts to file: {file}")
    rel_dir = f"{parts[-3]}/{parts[-2]}"
    features[rel_dir] = Feature(
        file=file,
        rel_dir=rel_dir,
        workflows=workflows,
        activities=activities,
        expect_activity_error=expect_activity_error,
        expect_run_result=expect_run_result,
        start=start,
        check_result=check_result,
    )


@dataclass
class Feature:
    file: str
    rel_dir: str  # Always relative to feature dir and uses forward slashes
    workflows: List[Type]
    activities: List[Callable]
    expect_activity_error: Optional[str]
    expect_run_result: Optional[Any]
    start: Optional[Callable[[Runner], Awaitable[WorkflowHandle]]]
    check_result: Optional[Callable[[Runner, WorkflowHandle], Awaitable[None]]]


class Runner:
    def __init__(
        self, *, address: str, namespace: str, task_queue: str, feature: Feature
    ) -> None:
        self.address = address
        self.namespace = namespace
        self.task_queue = task_queue
        self.feature = feature
        self.worker: Optional[Worker] = None
        self._worker_task: Optional[asyncio.Task] = None

    async def run(self) -> None:
        logger.info("Executing feature %s", self.feature.rel_dir)

        # Connect client
        self.client = await Client.connect(self.address, namespace=self.namespace)

        # Run worker
        self.start_worker()
        try:
            # Start and get handle
            handle: WorkflowHandle
            if self.feature.start:
                handle = await self.feature.start(self)
            else:
                handle = await self.start_single_parameterless_workflow()

            # Result check
            logger.debug("Checking result on feature %s", self.feature.rel_dir)
            if self.feature.check_result:
                await self.feature.check_result(self, handle)
            else:
                await self.check_result(handle)

            # TODO(cretz): History check
        finally:
            await self.stop_worker()

    async def start_single_parameterless_workflow(self) -> WorkflowHandle:
        if len(self.feature.workflows) != 1:
            raise ValueError("Must have a single workflow")
        defn = workflow._Definition.must_from_class(self.feature.workflows[0])
        return await self.client.start_workflow(
            defn.name,
            id=f"{self.feature.rel_dir}-{uuid.uuid4()}",
            task_queue=self.task_queue,
            execution_timeout=timedelta(minutes=1),
        )

    async def check_result(self, handle: WorkflowHandle) -> None:
        try:
            result = await handle.result()
            if self.feature.expect_run_result:
                assert result == self.feature.expect_run_result

        except Exception as err:
            if self.feature.expect_activity_error:
                if not isinstance(err, WorkflowFailureError):
                    raise TypeError("Expected activity error") from err
                elif not isinstance(err.cause, ActivityError):
                    raise TypeError("Expected activity error") from err
                elif not isinstance(err.cause.cause, ApplicationError):
                    raise TypeError("Expected activity error") from err
                elif err.cause.cause.message != self.feature.expect_activity_error:
                    raise TypeError("Unexpected activity error") from err
            else:
                raise err

    def start_worker(self):
        """Creates and starts worker with the task queue, workflows, and
        activities set."""
        if self.worker is not None:
            raise RuntimeError("Worker already started")
        self.worker = Worker(
            self.client,
            task_queue=self.task_queue,
            workflows=self.feature.workflows,
            activities=self.feature.activities,
        )
        self._worker_task = asyncio.create_task(self.worker.run())

    async def stop_worker(self):
        """Stop worker if running."""
        if self.worker is not None:
            try:
                await self.worker.shutdown()
                await self._worker_task
            finally:
                self.worker = None
                self._worker_task = None
