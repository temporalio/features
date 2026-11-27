namespace update.rejected_update_worker_restart;

using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;
using Xunit;

class Feature : IFeature
{
    [Workflow]
    class MyWorkflow
    {
        private int counter;
        private bool finished;

        [WorkflowRun]
        public async Task<int> RunAsync()
        {
            await Workflow.WaitConditionAsync(() => finished);
            return counter;
        }

        [WorkflowUpdate]
        public Task<int> AddAsync(int value)
        {
            counter += value;
            return Task.FromResult(counter);
        }

        [WorkflowUpdateValidator(nameof(AddAsync))]
        public void Validate(int value)
        {
            if (value < 0)
            {
                throw new ApplicationFailureException($"negative value: {value}");
            }
        }

        [WorkflowSignal]
        public Task FinishAsync()
        {
            finished = true;
            return Task.CompletedTask;
        }
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>();

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfUpdateNotSupportedAsync();
        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());

        await runner.WaitForEventAsync(handle,
            e => e.EventType == EventType.WorkflowTaskCompleted);
        var before = await handle.FetchHistoryAsync();

        await Assert.ThrowsAsync<WorkflowUpdateFailedException>(
            () => handle.ExecuteUpdateAsync(wf => wf.AddAsync(-1)));
        var after = await handle.FetchHistoryAsync();
        Assert.True(before.Events.SequenceEqual(after.Events), "rejection changed history");

        await runner.StopWorker();
        runner.StartWorker();

        Assert.Equal(1, await handle.ExecuteUpdateAsync(wf => wf.AddAsync(1)));
        await handle.SignalAsync(wf => wf.FinishAsync());
        return handle;
    }

    public async Task CheckResultAsync(Runner runner, WorkflowHandle handle) =>
        Assert.Equal(1, await handle.GetResultAsync<int>());

    public async Task CheckHistoryAsync(Runner runner, WorkflowHandle handle)
    {
        await runner.CheckCurrentHistoryAsync(handle);
        var history = await handle.FetchHistoryAsync();
        Assert.Single(history.Events.Where(e => e.EventType == EventType.WorkflowExecutionUpdateAccepted));
        Assert.Single(history.Events.Where(e => e.EventType == EventType.WorkflowExecutionUpdateCompleted));
        Assert.DoesNotContain(history.Events,
            e => e.EventType == EventType.WorkflowExecutionUpdateRejected);
    }
}
