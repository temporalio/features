namespace update.async_accepted;

using Temporalio.Activities;
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
        private bool shutdown;
        private bool finishUpdate;

        [WorkflowRun]
        public Task RunAsync() => Workflow.WaitConditionAsync(() => shutdown);

        [WorkflowSignal]
        public async Task ShutdownAsync() => shutdown = true;

        [WorkflowSignal]
        public async Task FinishUpdateAsync() => finishUpdate = true;

        [WorkflowUpdate]
        public async Task<int> SuccessfulUpdateAsync()
        {
            await Workflow.WaitConditionAsync(() => finishUpdate);
            finishUpdate = false;
            return 123;
        }

        [WorkflowUpdate]
        public async Task FailureUpdateAsync()
        {
            await Workflow.WaitConditionAsync(() => finishUpdate);
            finishUpdate = false;
            throw new ApplicationFailureException("Intentional failure");
        }
    }

    class MyActivities
    {
        [Activity]
        public static int MyActivity() => 6;
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>().AddAllActivities<MyActivities>(null);

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfAsyncUpdateNotSupportedAsync();

        // Start workflow
        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());

        // Start update
        var updateHandle1 = await handle.StartUpdateAsync(
            wf => wf.SuccessfulUpdateAsync(), new(WorkflowUpdateStage.Accepted));
        // Send signal to finish the update
        await handle.SignalAsync(wf => wf.FinishUpdateAsync());
        // Confirm result
        Assert.Equal(123, await updateHandle1.GetResultAsync());
        // Create another handle and confirm its result is the same
        Assert.Equal(123, await handle.GetUpdateHandle<int>(updateHandle1.Id).GetResultAsync());

        // Start a failed update
        var updateHandle2 = await handle.StartUpdateAsync(
            wf => wf.FailureUpdateAsync(), new(WorkflowUpdateStage.Accepted));
        // Send signal to finish the update
        await handle.SignalAsync(wf => wf.FinishUpdateAsync());
        // Confirm failure
        var exc = await Assert.ThrowsAsync<WorkflowUpdateFailedException>(
            () => updateHandle2.GetResultAsync());
        Assert.Equal("Intentional failure", exc.InnerException?.Message);

        // Start an update but cancel/timeout waiting on its result
        var updateHandle3 = await handle.StartUpdateAsync(
            wf => wf.SuccessfulUpdateAsync(), new(WorkflowUpdateStage.Accepted));
        // Wait for result only for 100ms
        using var tokenSource = new CancellationTokenSource();
        tokenSource.CancelAfter(TimeSpan.FromMilliseconds(100));
        await Assert.ThrowsAsync<OperationCanceledException>(() =>
            updateHandle3.GetResultAsync(new() { CancellationToken = tokenSource.Token }));

        await handle.SignalAsync(wf => wf.ShutdownAsync());
        return handle;
    }
}