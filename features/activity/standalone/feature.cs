namespace activity.standalone;

using Temporalio.Activities;
using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;

class Feature : IFeature
{
    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddAllActivities<MyActivities>(null);

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var handle = await runner.Client.StartActivityAsync(
            () => MyActivities.Echo("hello"),
            new(Guid.NewGuid().ToString(), runner.WorkerOptions.TaskQueue!)
            {
                StartToCloseTimeout = TimeSpan.FromMinutes(1),
            });

        Assert.Equal("hello", await handle.GetResultAsync());

        var desc = await handle.DescribeAsync();
        Assert.Equal(ActivityExecutionStatus.Completed, desc.Status);
        return null;
    }

    class MyActivities
    {
        [Activity]
        public static Task<string> Echo(string value) => Task.FromResult(value);
    }
}
