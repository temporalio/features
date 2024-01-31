namespace update.deduplication;

using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    [Workflow]
    class MyWorkflow
    {
        private int count;
        private bool shutdown;

        [WorkflowRun]
        public Task RunAsync() => Workflow.WaitConditionAsync(() => shutdown);

        [WorkflowUpdate]
        public async Task<int> MyUpdate(bool exit)
        {
            shutdown = exit;
            count++;
            return count;
        }

        [WorkflowQuery]
        public int GetCount() => count;
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options)
    {
        options.AddWorkflow<MyWorkflow>();
    }

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfUpdateNotSupportedAsync();

        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());

        var updateId = "myid";
        await handle.ExecuteUpdateAsync(wf => wf.MyUpdate(false), new() {UpdateID = updateId});
        Assert.Equal(1, await handle.QueryAsync(wf => wf.GetCount()));
        await handle.ExecuteUpdateAsync(wf => wf.MyUpdate(false), new() {UpdateID = updateId});
        Assert.Equal(1, await handle.QueryAsync(wf => wf.GetCount()));
        await handle.ExecuteUpdateAsync(wf => wf.MyUpdate(true));

        return handle;
    }
}