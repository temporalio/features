namespace Temporalio.Features.Harness;

using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Worker;

/// <summary>
/// Runner for running features.
/// </summary>
public class Runner
{
    private bool? maybeUpdateSupported;
    private ITemporalClient? client;
    private WorkerState? workerState;
    private CancellationToken? runCancelToken;

    class WorkerState
    {
        private readonly CancellationTokenSource workerStopper;
        private readonly Task workerTask;

        public WorkerState(
            CancellationTokenSource workerStopper,
            TemporalWorker worker
        )
        {
            this.workerStopper = workerStopper;
            workerTask = Task.Run(async () => await worker.ExecuteAsync(workerStopper.Token));
        }

        public async Task StopAndWait()
        {
            workerStopper.Cancel();
            try
            {
                await workerTask;
            }
            catch (OperationCanceledException)
            {
                // No good option here to not get an exception thrown when waiting on potentially
                // different worker instances.
            }
        }
    }


    internal Runner(
        TemporalClientConnectOptions clientConnectOptions,
        string taskQueue,
        PreparedFeature feature,
        ILoggerFactory loggerFactory,
        string? httpProxyUrl)
    {
        PreparedFeature = feature;
        Logger = loggerFactory.CreateLogger(PreparedFeature.FeatureType);
        Feature = (IFeature)Activator.CreateInstance(PreparedFeature.FeatureType, true)!;
        HttpProxyUrl = httpProxyUrl;

        ClientOptions = (TemporalClientConnectOptions)clientConnectOptions.Clone();
        Feature.ConfigureClient(this, ClientOptions);
        WorkerOptions = new(taskQueue) { LoggerFactory = loggerFactory };
    }

    public ITemporalClient Client => client!;

    public IFeature Feature { get; private init; }

    public ILogger Logger { get; private init; }

    public PreparedFeature PreparedFeature { get; private init; }

    public TemporalClientConnectOptions ClientOptions { get; private init; }

    public TemporalWorkerOptions WorkerOptions { get; private init; }

    public string? HttpProxyUrl { get; private init; }

    /// <summary>
    /// Run the feature with the given cancellation token.
    /// </summary>
    /// <param name="cancellationToken"></param>
    /// <returns></returns>
    public async Task RunAsync(CancellationToken cancellationToken)
    {
        Logger.LogInformation("Executing feature {Feature}", PreparedFeature.Dir);
        runCancelToken = cancellationToken;
        client = await TemporalClient.ConnectAsync(ClientOptions);
        Feature.ConfigureWorker(this, WorkerOptions);
        StartWorker();

        var run = await Feature.ExecuteAsync(this);
        if (run == null)
        {
            Logger.LogInformation("Feature {Feature} returned null", PreparedFeature.Dir);
            return;
        }

        Logger.LogInformation("Checking result of feature {Feature}", PreparedFeature.Dir);
        await Feature.CheckResultAsync(this, run);
        await Feature.CheckHistoryAsync(this, run);

        if (workerState != null)
        {
            await workerState.StopAndWait();
        }
    }

    /// <summary>
    /// Expects a single parameterless workflow on the worker and starts it.
    /// </summary>
    /// <returns>Workflow handle for the started run.</returns>
    public Task<WorkflowHandle> StartSingleParameterlessWorkflowAsync()
    {
        var workflow = WorkerOptions.Workflows.SingleOrDefault() ??
                       throw new InvalidOperationException("Must have a single workflow");
        return Client.StartWorkflowAsync(workflow.Name!, Array.Empty<object?>(),
            NewWorkflowOptions());
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
            await new WorkflowReplayer(replayerOptions).ReplayWorkflowAsync(
                await handle.FetchHistoryAsync());
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
                "does-not-exist",
                Array.Empty<object?>(),
                new(WorkflowUpdateStage.Accepted)));

    /// <summary>
    /// Start the worker.
    /// </summary>
    public void StartWorker()
    {
        if (workerState != null)
        {
            throw new InvalidOperationException("Worker already started");
        }

        workerState = new(
            CancellationTokenSource.CreateLinkedTokenSource(
                runCancelToken ?? CancellationToken.None),
            new TemporalWorker(Client, WorkerOptions)
        );
    }


    /// <summary>
    /// Stop the worker.
    /// </summary>
    public async Task StopWorker()
    {
        if (workerState != null)
        {
            await workerState.StopAndWait();
            workerState = null;
        }
    }

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
                e.Code == RpcException.StatusCode.Unimplemented ||
                e.Code == RpcException.StatusCode.PermissionDenied)
            {
                // Not implemented or permission denied means not supported,
                // everything else is an error
                maybeUpdateSupported = false;
            }
        }

        return maybeUpdateSupported.Value;
    }

    /// <summary>
    /// Wait for a specific event in the workflow history.
    /// </summary>
    /// <param name="handle">Workflow handle.</param>
    /// <param name="predicate">Predicate to check for the event.</param>
    /// <param name="timeout">Timeout for waiting.</param>
    /// <returns>Task for completion.</returns>
    public async Task WaitForEventAsync(WorkflowHandle handle, Func<Temporalio.Api.History.V1.HistoryEvent, bool> predicate, TimeSpan? timeout = null)
    {
        timeout ??= TimeSpan.FromSeconds(30);
        var start = DateTime.UtcNow;
        var pollInterval = TimeSpan.FromMilliseconds(100);
        
        while (DateTime.UtcNow - start < timeout)
        {
            var history = await handle.FetchHistoryAsync();
            var foundEvent = history.Events.FirstOrDefault(predicate);
            if (foundEvent != null)
            {
                return;
            }
            await Task.Delay(pollInterval);
        }
        
        throw new TimeoutException($"Event not found within {timeout.Value.TotalMilliseconds}ms");
    }

    /// <summary>
    /// Wait for an activity task scheduled event.
    /// </summary>
    /// <param name="handle">Workflow handle.</param>
    /// <param name="timeout">Timeout for waiting.</param>
    /// <returns>Task for completion.</returns>
    public async Task WaitForActivityTaskScheduledAsync(WorkflowHandle handle, TimeSpan? timeout = null)
    {
        await WaitForEventAsync(handle, 
            e => e.EventType == Temporalio.Api.Enums.V1.EventType.ActivityTaskScheduled, 
            timeout);
    }
}