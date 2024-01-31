namespace update.worker_restart;

using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    // Don't do this. We do here because we need to fail the workflow task a controlled # of times.
    static SemaphoreSlim activityStarted = new(0);
    static SemaphoreSlim finishActivity = new(0);

    [Workflow]
    class MyWorkflow
    {
        private bool shutdown;

        [WorkflowRun]
        public async Task RunAsync()
        {
            await Workflow.WaitConditionAsync(() => shutdown);
        }

        [WorkflowUpdate]
        public async Task UpdateMe()
        {
            await Workflow.ExecuteActivityAsync((MyActivities act) => act.MyActivity(),
                new() { StartToCloseTimeout = TimeSpan.FromSeconds(30) });
            shutdown = true;
        }
    }

    class MyActivities
    {
        [Activity]
        public async Task MyActivity()
        {
            activityStarted.Release();
            await finishActivity.WaitAsync(ActivityExecutionContext.Current.CancellationToken);
        }
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>().AddAllActivities(new MyActivities());

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfUpdateNotSupportedAsync();
        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());

        var updateTask = handle.ExecuteUpdateAsync(wf => wf.UpdateMe());

        await activityStarted.WaitAsync();
        await runner.StopWorker();
        runner.StartWorker();
        finishActivity.Release();

        await updateTask;
        return handle;
    }
}