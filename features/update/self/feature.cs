namespace update.self;

using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    [Workflow]
    class MyWorkflow
    {
        private bool shutdown;

        [WorkflowRun]
        public async Task RunAsync()
        {
            await Workflow.ExecuteActivityAsync((MyActivities act) => act.MyActivity(),
                new() { StartToCloseTimeout = TimeSpan.FromSeconds(5) });
            await Workflow.WaitConditionAsync(() => shutdown);
        }

        [WorkflowUpdate]
        public async Task UpdateMe()
        {
            shutdown = true;
        }
    }

    class MyActivities
    {
        public MyActivities(ITemporalClient client)
        {
            Client = client;
        }

        private ITemporalClient Client { get; }

        [Activity]
        public async Task MyActivity()
        {
            var handle =
                Client.GetWorkflowHandle<MyWorkflow>(ActivityExecutionContext.Current.Info
                    .WorkflowId!);
            await handle.ExecuteUpdateAsync(wf => wf.UpdateMe());
        }
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>().AddAllActivities<MyActivities>(new(runner.Client));

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfUpdateNotSupportedAsync();
        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());
        await handle.ExecuteUpdateAsync(wf => wf.UpdateMe());
        return handle;
    }
}