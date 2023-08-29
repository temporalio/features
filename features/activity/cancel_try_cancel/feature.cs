namespace activity.cancel_try_cancel;

using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>().AddAllActivities(new MyActivities(runner.Client));

    [Workflow]
    class MyWorkflow
    {
        private string? activityResult;

        [WorkflowRun]
        public async Task RunAsync()
        {
            // Create token to cancel
            using var activityCancel = CancellationTokenSource.CreateLinkedTokenSource(Workflow.CancellationToken);

            // Start activity
            var activityTask = Workflow.ExecuteActivityAsync(
                (MyActivities act) => act.CancellableActivity(),
                new()
                {
                    ScheduleToCloseTimeout = TimeSpan.FromMinutes(1),
                    HeartbeatTimeout = TimeSpan.FromSeconds(5),
                    RetryPolicy = new() { MaximumAttempts = 1 },
                    CancellationType = ActivityCancellationType.TryCancel,
                    CancellationToken = activityCancel.Token,
                });

            // Sleep for short time (force task turnover)
            await Workflow.DelayAsync(1);

            // Cancel and confirm the activity errors with the cancel
            activityCancel.Cancel();
            try
            {
                await activityTask;
                throw new ApplicationFailureException("Activity should have thrown cancellation error");
            }
            catch (ActivityFailureException e) when (e.InnerException is CanceledFailureException)
            {
            }

            // Confirm signal is cancelled
            await Workflow.WaitConditionAsync(() => activityResult is not null);
            if (activityResult != "cancelled")
            {
                throw new ApplicationFailureException($"Expected cancelled, got {activityResult}");
            }
        }

        [WorkflowSignal]
        public async Task SetActivityResultAsync(string res) => activityResult = res;
    }

    class MyActivities
    {
        private readonly ITemporalClient client;

        public MyActivities(ITemporalClient client) => this.client = client;

        [Activity]
        public async Task CancellableActivity()
        {
            // Heartbeat every second for a minute
            var result = "timeout";
            try
            {
                for (int i = 0; i < 60; i++)
                {
                    await Task.Delay(1000, ActivityExecutionContext.Current.CancellationToken);
                    ActivityExecutionContext.Current.Heartbeat();
                }
            }
            catch (OperationCanceledException)
            {
                result = "cancelled";
            }

            // Send result as signal to workflow
            await client.GetWorkflowHandle<MyWorkflow>(ActivityExecutionContext.Current.Info.WorkflowId).
                SignalAsync(wf => wf.SetActivityResultAsync(result));
        }
    }
}