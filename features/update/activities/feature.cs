namespace update.activities;

using Temporalio.Activities;
using Temporalio.Client;
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

        [WorkflowRun]
        public Task RunAsync() => Workflow.WaitConditionAsync(() => shutdown);

        [WorkflowSignal]
        public async Task ShutdownAsync() => shutdown = true;

        [WorkflowUpdate]
        public Task<int> DoActivitiesAsync() =>
            // Run 5 activities and sum the results
            Task.WhenAll(
                Enumerable.Range(0, 5).Select(_ =>
                    Workflow.ExecuteActivityAsync(
                        () => MyActivities.MyActivity(),
                        new() { StartToCloseTimeout = TimeSpan.FromSeconds(5) }))).
                ContinueWith(vals => Enumerable.Sum(vals.Result));
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
        await runner.SkipIfUpdateNotSupportedAsync();
        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());
        Assert.Equal(30, await handle.ExecuteUpdateAsync(wf => wf.DoActivitiesAsync()));
        await handle.SignalAsync(wf => wf.ShutdownAsync());
        return handle;
    }
}