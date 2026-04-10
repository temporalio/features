namespace schedule.duplicate_error;

using Temporalio.Client;
using Temporalio.Client.Schedules;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    [Workflow]
    class MyWorkflow
    {
        [WorkflowRun]
        public Task RunAsync() => Task.CompletedTask;
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>();

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var scheduleId = $"schedule-duplicate-error-{Guid.NewGuid()}";
        var schedule = new Schedule(
            Action: ScheduleActionStartWorkflow.Create(
                (MyWorkflow wf) => wf.RunAsync(),
                new(id: $"wf-{Guid.NewGuid()}", taskQueue: runner.WorkerOptions.TaskQueue!)),
            Spec: new()
            {
                Intervals = new List<ScheduleIntervalSpec>
                {
                    new(Every: TimeSpan.FromHours(1))
                },
            })
        {
            State = new() { Paused = true },
        };

        var handle = await runner.Client.CreateScheduleAsync(scheduleId, schedule);

        try
        {
            // Creating again with the same schedule ID should throw ScheduleAlreadyRunningException.
            await Assert.ThrowsAsync<ScheduleAlreadyRunningException>(
                () => runner.Client.CreateScheduleAsync(scheduleId, schedule));
        }
        finally
        {
            await handle.DeleteAsync();
        }

        return null;
    }
}
