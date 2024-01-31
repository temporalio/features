namespace update.validation_replay;

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
            if (failCounter == 0)
            {
                failCounter++;
                throw new Exception("I'll fail the task");
            }

            shutdown = true;
        }

        [WorkflowUpdateValidator(nameof(UpdateMe))]
        public void ValidateUpdate()
        {
            // We will start rejecting things once we've failed the task, and hence are now
            // replaying. The fact that the workflow completes demonstrates that even though the
            // validator would "reject" on replay, it doesn't even run, since the update has already
            // been accepted.
            if (failCounter > 1)
            {
                throw new Exception("I would reject if I ever ran");
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

        await handle.ExecuteUpdateAsync(wf => wf.UpdateMe());
        await handle.GetResultAsync();

        Assert.Equal(1, failCounter);

        return handle;
    }
}