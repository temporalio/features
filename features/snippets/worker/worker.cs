using Temporalio.Client;
using Temporalio.Worker;

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
}
