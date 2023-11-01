namespace Temporalio.Features.Harness;

using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Worker;
using Temporalio.Workflows;

/// <summary>
/// Runner for running features.
/// </summary>
public class Runner
{
    private bool? maybeUpdateSupported;

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
        return Client.StartWorkflowAsync(workflow.Name!, Array.Empty<object?>(), NewWorkflowOptions());
    }

    public WorkflowOptions NewWorkflowOptions() =>
        new(id: $"{PreparedFeature.Dir}-{Guid.NewGuid()}", taskQueue: WorkerOptions.TaskQueue!)
        {
            ExecutionTimeout = TimeSpan.FromMinutes(1)
        };

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

    /// <summary>
    /// Throw skip exception if update not supported.
    /// </summary>
    /// <returns>Task for completion.</returns>
    /// <exception cref="TestSkippedException">If update not supported.</exception>
    public async Task SkipIfUpdateNotSupportedAsync()
    {
        if (await CheckUpdateSupportedAsync())
        {
            return;
        }
        throw new TestSkippedException("Update not supported");
    }

    /// <summary>
    /// Check if update not supported.
    /// </summary>
    /// <returns>True if supported, false if not.</returns>
    public Task<bool> CheckUpdateSupportedAsync() =>
        CheckUpdateSupportCallAsync(() =>
            Client.GetWorkflowHandle("does-not-exist").ExecuteUpdateAsync(
                "does-not-exist", Array.Empty<object?>()));

    /// <summary>
    /// Throw skip exception if async update not supported.
    /// </summary>
    /// <returns>Task for completion.</returns>
    /// <exception cref="TestSkippedException">If async update not supported.</exception>
    public async Task SkipIfAsyncUpdateNotSupportedAsync()
    {
        if (await CheckAsyncUpdateSupportedAsync())
        {
            return;
        }
        throw new TestSkippedException("Async update not supported");
    }

    /// <summary>
    /// Check if async update not supported.
    /// </summary>
    /// <returns>True if supported, false if not.</returns>
    public Task<bool> CheckAsyncUpdateSupportedAsync() =>
        CheckUpdateSupportCallAsync(() =>
            Client.GetWorkflowHandle("does-not-exist").StartUpdateAsync(
                "does-not-exist", Array.Empty<object?>()));

    private async Task<bool> CheckUpdateSupportCallAsync(Func<Task> failingFunc)
    {
        // Don't care about races
        if (maybeUpdateSupported == null)
        {
            try
            {
                try
                {
                    await failingFunc();
                    throw new InvalidOperationException("Unexpected success");
                }
                catch (AggregateException e)
                {
                    // Bug with agg exception: https://github.com/temporalio/sdk-dotnet/issues/151
                    throw e.InnerExceptions.Single();
                }
            }
            catch (RpcException e) when (e.Code == RpcException.StatusCode.NotFound)
            {
                // Not found workflow means update does exist
                maybeUpdateSupported = true;
            }
            catch (RpcException e) when (
                e.Code == RpcException.StatusCode.Unimplemented || e.Code == RpcException.StatusCode.PermissionDenied)
            {
                // Not implemented or permission denied means not supported,
                // everything else is an error
                maybeUpdateSupported = false;
            }
        }
        return maybeUpdateSupported.Value;
    }
}