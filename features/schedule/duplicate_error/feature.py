import uuid
from datetime import timedelta

from temporalio import workflow
from temporalio.client import (
    Schedule,
    ScheduleActionStartWorkflow,
    ScheduleAlreadyRunningError,
    ScheduleIntervalSpec,
    ScheduleSpec,
    ScheduleState,
    WorkflowHandle,
)

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> None:
        pass


async def start(runner: Runner) -> WorkflowHandle:
    schedule_id = f"schedule-duplicate-error-{uuid.uuid4()}"
    handle = await runner.client.create_schedule(
        schedule_id,
        Schedule(
            action=ScheduleActionStartWorkflow(
                Workflow.run,
                id=f"wf-{uuid.uuid4()}",
                task_queue=runner.task_queue,
            ),
            spec=ScheduleSpec(
                intervals=[ScheduleIntervalSpec(every=timedelta(hours=1))]
            ),
            state=ScheduleState(paused=True),
        ),
    )

    try:
        # Creating again with the same schedule ID should raise ScheduleAlreadyRunningError.
        raised = False
        try:
            await runner.client.create_schedule(
                schedule_id,
                Schedule(
                    action=ScheduleActionStartWorkflow(
                        Workflow.run,
                        id=f"wf-{uuid.uuid4()}",
                        task_queue=runner.task_queue,
                    ),
                    spec=ScheduleSpec(
                        intervals=[ScheduleIntervalSpec(every=timedelta(hours=1))]
                    ),
                    state=ScheduleState(paused=True),
                ),
            )
        except ScheduleAlreadyRunningError:
            raised = True

        assert raised, "expected ScheduleAlreadyRunningError"
    finally:
        await handle.delete()

    return await runner.start_single_parameterless_workflow()


register_feature(
    workflows=[Workflow],
    start=start,
)
