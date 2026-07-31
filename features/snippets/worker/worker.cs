using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Common;
using Temporalio.Worker;
using Temporalio.Workflows;

public class WorkerSnippet
{
    public static async Task Run()
    {
        var client = await TemporalClient.ConnectAsync(new("localhost:7233"));

        // @@@SNIPSTART dotnet-worker-max-cached-workflows
        using var worker = new TemporalWorker(
            client,
            new TemporalWorkerOptions("task-queue")
            {
                MaxCachedWorkflows = 0
            });
        // @@@SNIPEND
    }

    public static async Task CreateWorker()
    {
        var client = await TemporalClient.ConnectAsync(new("localhost:7233"));

        // @@@SNIPSTART dotnet-create-worker
        var options = new TemporalWorkerOptions("my-task-queue");
        options.AddWorkflow<GreetingWorkflow>();
        options.AddAllActivities(typeof(GreetingActivities), null);

        using var worker = new TemporalWorker(client, options);
        await worker.ExecuteAsync(CancellationToken.None);
        // @@@SNIPEND
    }

    public static async Task CreateVersionedWorker()
    {
        var client = await TemporalClient.ConnectAsync(new("localhost:7233"));

        // @@@SNIPSTART dotnet-versioned-worker
        var options = new TemporalWorkerOptions("my-task-queue")
        {
            DeploymentOptions = new WorkerDeploymentOptions(
                new WorkerDeploymentVersion("my-app", "1.0"),
                useWorkerVersioning: true),
        };
        options.AddWorkflow<VersionedGreetingWorkflow>();
        options.AddAllActivities(typeof(GreetingActivities), null);

        using var worker = new TemporalWorker(client, options);
        // @@@SNIPEND
        await Task.CompletedTask;
    }

    public static async Task ShutdownWorker()
    {
        var client = await TemporalClient.ConnectAsync(new("localhost:7233"));

        // @@@SNIPSTART dotnet-worker-graceful-shutdown
        using var tokenSource = new CancellationTokenSource();
        Console.CancelKeyPress += (_, eventArgs) =>
        {
            tokenSource.Cancel();
            eventArgs.Cancel = true;
        };

        var options = new TemporalWorkerOptions("my-task-queue")
        {
            GracefulShutdownTimeout = TimeSpan.FromSeconds(30),
        };
        options.AddWorkflow<GreetingWorkflow>();

        using var worker = new TemporalWorker(client, options);
        await worker.ExecuteAsync(tokenSource.Token);
        // @@@SNIPEND
    }

    public static class GreetingActivities
    {
        [Activity]
        public static string SayHello(string name) => $"Hello, {name}!";
    }

    [Workflow]
    public class GreetingWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync(string name) =>
            await Workflow.ExecuteActivityAsync(
                () => GreetingActivities.SayHello(name),
                new() { StartToCloseTimeout = TimeSpan.FromSeconds(10) });
    }

    // A versioning behavior is only valid on a Worker that has versioning enabled.
    [Workflow(VersioningBehavior = VersioningBehavior.Pinned)]
    public class VersionedGreetingWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync(string name) =>
            await Workflow.ExecuteActivityAsync(
                () => GreetingActivities.SayHello(name),
                new() { StartToCloseTimeout = TimeSpan.FromSeconds(10) });
    }
}
