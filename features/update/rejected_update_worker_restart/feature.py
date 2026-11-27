from temporalio import workflow
from temporalio.api.enums.v1 import EventType
from temporalio.client import WorkflowHandle, WorkflowUpdateFailedError
from temporalio.exceptions import ApplicationError

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    def __init__(self) -> None:
        self.counter = 0
        self.finished = False

    @workflow.run
    async def run(self) -> int:
        await workflow.wait_condition(lambda: self.finished)
        return self.counter

    @workflow.update
    def add(self, value: int) -> int:
        self.counter += value
        return self.counter

    @add.validator
    def validate(self, value: int) -> None:
        if value < 0:
            raise ApplicationError(f"negative value: {value}")

    @workflow.signal
    def finish(self) -> None:
        self.finished = True


async def start(runner: Runner) -> WorkflowHandle:
    await runner.skip_if_update_unsupported()
    return await runner.start_single_parameterless_workflow()


async def check_result(runner: Runner, handle: WorkflowHandle) -> None:
    await runner.wait_for_event(
        handle,
        lambda event: event.event_type == EventType.EVENT_TYPE_WORKFLOW_TASK_COMPLETED,
    )
    before = [event async for event in handle.fetch_history_events()]

    try:
        await handle.execute_update(Workflow.add, -1)
    except WorkflowUpdateFailedError:
        pass
    else:
        raise AssertionError("negative Update was accepted")

    after = [event async for event in handle.fetch_history_events()]
    assert before == after, "rejection changed history"

    await runner.stop_worker()
    runner.start_worker()

    assert await handle.execute_update(Workflow.add, 1) == 1
    await handle.signal(Workflow.finish)
    assert await handle.result() == 1

    events = [event async for event in handle.fetch_history_events()]
    assert (
        sum(
            event.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_ACCEPTED
            for event in events
        )
        == 1
    )
    assert (
        sum(
            event.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_COMPLETED
            for event in events
        )
        == 1
    )
    assert not any(
        event.event_type == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_REJECTED
        for event in events
    )


register_feature(workflows=[Workflow], start=start, check_result=check_result)
