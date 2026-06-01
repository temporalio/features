namespace worker_shutdown.poll_complete_on_shutdown;

using System.Text.Json;
using Temporalio.Activities;
using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Common;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;
using Xunit;

class Feature : IFeature
{
    private const int WorkflowCount = 5;
    private static readonly TimeSpan HistoryTimeout = TimeSpan.FromSeconds(15);

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options)
    {
        options.AddWorkflow<MyWorkflow>().AddAllActivities(new MyActivities());
        options.GracefulShutdownTimeout = TimeSpan.FromSeconds(10);
    }

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var handles = new List<WorkflowHandle>();
        try
        {
            for (var i = 0; i < WorkflowCount; i++)
            {
                var options = runner.NewWorkflowOptions();
                options.TaskTimeout = TimeSpan.FromSeconds(5);
                handles.Add(await runner.Client.StartWorkflowAsync(
                    (MyWorkflow wf) => wf.RunAsync(),
                    options));
            }

            foreach (var handle in handles)
            {
                await runner.WaitForActivityTaskScheduledAsync(handle, TimeSpan.FromSeconds(10));
            }

            var start = DateTime.UtcNow;
            await runner.StopWorker();
            Assert.True(DateTime.UtcNow - start <= TimeSpan.FromSeconds(5));

            if (ExpectWorkerPollCompleteOnShutdown())
            {
                foreach (var handle in handles)
                {
                    var history = await handle.FetchHistoryAsync();
                    Assert.DoesNotContain(
                        history.Events,
                        e => e.EventType == EventType.WorkflowTaskFailed || e.EventType == EventType.WorkflowTaskTimedOut);
                }
            }
            else
            {
                await AssertAnyWorkflowTaskProblemAsync(handles);
            }
        }
        finally
        {
            foreach (var handle in handles)
            {
                try
                {
                    await handle.TerminateAsync("feature cleanup");
                }
                catch
                {
                    // Ignore cleanup races.
                }
            }
        }

        return null;
    }

    private static bool ExpectWorkerPollCompleteOnShutdown()
    {
        var capabilitiesJson = Environment.GetEnvironmentVariable("FEATURE_NAMESPACE_CAPABILITIES");
        if (string.IsNullOrWhiteSpace(capabilitiesJson))
        {
            throw new InvalidOperationException("FEATURE_NAMESPACE_CAPABILITIES is required");
        }
        var capabilities = JsonSerializer.Deserialize<Dictionary<string, bool>>(capabilitiesJson)
            ?? throw new InvalidOperationException("FEATURE_NAMESPACE_CAPABILITIES is invalid");
        if (!capabilities.TryGetValue("workerPollCompleteOnShutdown", out var value))
        {
            throw new InvalidOperationException(
                "FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown");
        }
        return value;
    }

    private static async Task AssertAnyWorkflowTaskProblemAsync(IEnumerable<WorkflowHandle> handles)
    {
        var deadline = DateTime.UtcNow + HistoryTimeout;
        while (DateTime.UtcNow < deadline)
        {
            foreach (var handle in handles)
            {
                var history = await handle.FetchHistoryAsync();
                if (history.Events.Any(
                    e => e.EventType == EventType.WorkflowTaskFailed || e.EventType == EventType.WorkflowTaskTimedOut))
                {
                    return;
                }
            }

            await Task.Delay(TimeSpan.FromMilliseconds(200));
        }

        throw new TimeoutException($"Expected a workflow task failure or timeout within {HistoryTimeout}");
    }

    [Workflow]
    class MyWorkflow
    {
        [WorkflowRun]
        public async Task RunAsync()
        {
            var options = new ActivityOptions
            {
                ScheduleToCloseTimeout = TimeSpan.FromSeconds(10),
                StartToCloseTimeout = TimeSpan.FromSeconds(5),
                RetryPolicy = new RetryPolicy { MaximumAttempts = 1 },
            };
            while (true)
            {
                await Workflow.DelayAsync(TimeSpan.FromMilliseconds(20));
                await Workflow.ExecuteActivityAsync((MyActivities act) => act.NoopAsync(), options);
            }
        }
    }

    class MyActivities
    {
        [Activity]
        public Task NoopAsync() => Task.CompletedTask;
    }
}
