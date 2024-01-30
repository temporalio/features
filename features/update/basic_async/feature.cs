using Temporalio.Exceptions;

namespace update.basic_async;

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
        public Task RunAsync() => Workflow.WaitConditionAsync(() => shutdown);

        [WorkflowUpdate]
        public async Task<string> MyUpdate(string _)
        {
            shutdown = true;
            return "hi";
        }

        [WorkflowUpdateValidator(nameof(MyUpdate))]
        public void ValidateMyUpdate(string arg)
        {
            if (arg == "invalid")
            {
                throw new ApplicationFailureException("invalid");
            }
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

        var badUpdateHandle = await handle.StartUpdateAsync(wf => wf.MyUpdate("invalid"));

        try
        {
            await badUpdateHandle.GetResultAsync();
        }
        catch (WorkflowUpdateFailedException)
        {
            // Expected
        }

        var goodUpdateHandle = await handle.StartUpdateAsync(wf => wf.MyUpdate("valid"));
        Assert.Equal("hi", await goodUpdateHandle.GetResultAsync());

        return handle;
    }
}
