using Temporalio.Exceptions;

namespace update.basic;

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

        try
        {
            await handle.ExecuteUpdateAsync(wf => wf.MyUpdate("invalid"));
        }
        catch (WorkflowUpdateFailedException)
        {
            // Expected
        }

        Assert.Equal("hi", await handle.ExecuteUpdateAsync(wf => wf.MyUpdate("valid")));
        return handle;
    }
}
