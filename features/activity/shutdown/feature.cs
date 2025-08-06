namespace activity.shutdown;

using System;
using System.Threading;
using System.Threading.Tasks;
using Temporalio.Activities;
using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;
using Xunit;

class Feature : IFeature
{
    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options)
    {
        options.AddWorkflow<MyWorkflow>().AddAllActivities(new MyActivities());
        options.GracefulShutdownTimeout = TimeSpan.FromSeconds(1);
    }

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var handle = await runner.StartSingleParameterlessWorkflowAsync();
        await Task.Delay(100);
        await runner.StopWorker();
        runner.StartWorker();
        return handle;
    }

    [Workflow]
    class MyWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync()
        {
            var options = new ActivityOptions
            {
                ScheduleToCloseTimeout = TimeSpan.FromMilliseconds(300),
                RetryPolicy = new() { MaximumAttempts = 1 },
            };

            var fut = Workflow.ExecuteActivityAsync((MyActivities act) => act.CancelSuccess(), options);
            var fut1 = Workflow.ExecuteActivityAsync((MyActivities act) => act.CancelFailure(), options);
            var fut2 = Workflow.ExecuteActivityAsync((MyActivities act) => act.CancelIgnore(), options);

            await fut;

            var exc1 = await Assert.ThrowsAsync<ActivityFailureException>(() => fut1);
            Assert.Contains("worker is shutting down", exc1.InnerException?.Message);

            var exc2 = await Assert.ThrowsAsync<ActivityFailureException>(() => fut2);
            var timeoutFailure = Assert.IsType<TimeoutFailureException>(exc2.InnerException);
            Assert.Equal(TimeoutType.ScheduleToClose, timeoutFailure.TimeoutType);

            return "done";
        }
    }

    class MyActivities
    {
        [Activity]
        public async Task CancelSuccess()
        {
            try
            {
                await Task.Delay(Timeout.Infinite, ActivityExecutionContext.Current.WorkerShutdownToken);
            }
            catch (OperationCanceledException)
            {
            }
        }

        [Activity]
        public async Task CancelFailure()
        {
            try
            {
                await Task.Delay(Timeout.Infinite, ActivityExecutionContext.Current.WorkerShutdownToken);
            }
            catch (OperationCanceledException)
            {
                throw new ApplicationFailureException("worker is shutting down");
            }
        }

        [Activity]
        public async Task CancelIgnore()
        {
            await Task.Delay(TimeSpan.FromSeconds(15));
        }
    }
}