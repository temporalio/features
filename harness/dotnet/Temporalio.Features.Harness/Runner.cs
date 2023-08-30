namespace Temporalio.Features.Harness;

using Temporalio.Client;
using Temporalio.Worker;

/// <summary>
/// Runner for running features.
/// </summary>
public class Runner
{
    internal Runner(
        ITemporalClient client,
        string taskQueue,
        PreparedFeature feature,
        ILoggerFactory loggerFactory)
    {
        Client = client;
        PreparedFeature = feature;
        Feature = (IFeature)Activator.CreateInstance(PreparedFeature.FeatureType, true)!;
        Logger = loggerFactory.CreateLogger(PreparedFeature.FeatureType);
        WorkerOptions = new(taskQueue) { LoggerFactory = loggerFactory };
        Feature.ConfigureWorker(this, WorkerOptions);
    }

    public ITemporalClient Client { get; private init; }

    public IFeature Feature { get; private init; }

    public ILogger Logger { get; private init; }

    public PreparedFeature PreparedFeature { get; private init; }

    public TemporalWorkerOptions WorkerOptions { get; private init; }

    /// <summary>
    /// Run the feature with the given cancellation token.
    /// </summary>
    /// <param name="cancellationToken"></param>
    /// <returns></returns>
    public async Task RunAsync(CancellationToken cancellationToken)
    {
        // Run inside worker
        Logger.LogInformation("Executing feature {Feature}", PreparedFeature.Dir);
        using var worker = new TemporalWorker(Client, WorkerOptions);
        await worker.ExecuteAsync(async () =>
        {
            var run = await Feature.ExecuteAsync(this);
            if (run == null)
            {
                Logger.LogInformation("Feature {Feature} returned null", PreparedFeature.Dir);
                return;
            }
            Logger.LogInformation("Checking result of feature {Feature}", PreparedFeature.Dir);
            await Feature.CheckResultAsync(this, run);
            await Feature.CheckHistoryAsync(this, run);
        }, cancellationToken);
    }

    /// <summary>
    /// Expects a single parameterless workflow on the worker and starts it.
    /// </summary>
    /// <returns>Workflow handle for the started run.</returns>
    public Task<WorkflowHandle> StartSingleParameterlessWorkflowAsync()
    {
        var workflow = WorkerOptions.Workflows.SingleOrDefault() ??
            throw new InvalidOperationException("Must have a single workflow");
        return Client.StartWorkflowAsync(
            workflow.Name!,
            Array.Empty<object?>(),
            new(id: $"{PreparedFeature.Dir}-{Guid.NewGuid()}", taskQueue: WorkerOptions.TaskQueue!)
            {
                ExecutionTimeout = TimeSpan.FromMinutes(1)
            });
    }

    /// <summary>
    /// Checks the current history for the given handle using the replayer.
    /// </summary>
    /// <param name="handle">Workflow handle.</param>
    /// <returns>Task for completion.</returns>
    public async Task CheckCurrentHistoryAsync(WorkflowHandle handle)
    {
        Logger.LogInformation("Checking current history of feature {Feature}", PreparedFeature.Dir);
        // Grab the history and replay
        var replayerOptions = new WorkflowReplayerOptions()
        {
            LoggerFactory = WorkerOptions.LoggerFactory!,
            Namespace = Client.Options.Namespace,
            TaskQueue = WorkerOptions.TaskQueue!,
        };
        foreach (var workflow in WorkerOptions.Workflows)
        {
            replayerOptions.AddWorkflow(workflow);
        }
        try
        {
            await new WorkflowReplayer(replayerOptions).ReplayWorkflowAsync(await handle.FetchHistoryAsync());
        }
        catch (Exception e)
        {
            throw new InvalidOperationException("Replay failed", e);
        }
    }
}