using Temporalio.Api.Enums.V1;

namespace update.non_durable_reject;

using Temporalio.Exceptions;
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

        for (var i = 0; i < 5; i++)
        {
            try
            {
                await handle.ExecuteUpdateAsync(wf => wf.MyUpdate("invalid"));
            }
            catch (WorkflowUpdateFailedException)
            {
                // Expected
            }
        }

        await handle.ExecuteUpdateAsync(wf => wf.MyUpdate("valid"));
        await handle.GetResultAsync();

        // Verify there are no rejections written to history
        var history = await handle.FetchHistoryAsync();
        Assert.DoesNotContain(history.Events,
            e => e.EventType == EventType.WorkflowExecutionUpdateRejected);

        return handle;
    }
}