namespace update.task_failure;

using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;
using Temporalio.Exceptions;

class Feature : IFeature
{
    // Don't do this. We do here because we need to fail the workflow task a controlled # of times.
    static int failCounter;

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
            if (failCounter < 2)
            {
                failCounter++;
                throw new Exception("I'll fail the task");
            }

            throw new ApplicationFailureException("I'll fail the update");
        }

        [WorkflowUpdate]
        public async Task ThrowOrEnd(bool _)
        {
            shutdown = true;
        }

        [WorkflowUpdateValidator(nameof(ThrowOrEnd))]
        public void ValidateUpdate(bool doShutdown)
        {
            if (!doShutdown)
            {
                throw new Exception("this will fail validation, not task");
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
            await handle.ExecuteUpdateAsync(wf => wf.UpdateMe());
        }
        catch (WorkflowUpdateFailedException)
        {
            // Expected
        }

        try
        {
            await handle.ExecuteUpdateAsync(wf => wf.ThrowOrEnd(false));
        }
        catch (WorkflowUpdateFailedException)
        {
            // Expected
        }

        await handle.ExecuteUpdateAsync(wf => wf.ThrowOrEnd(true));
        await handle.GetResultAsync();

        Assert.Equal(2, failCounter);

        return handle;
    }
}