namespace Temporalio.Features.Harness.Tests;

using System.Reflection;
using Microsoft.Extensions.Logging.Abstractions;
using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Testing;
using Temporalio.Worker;
using Xunit;

public class RunnerTests : IAsyncLifetime
{
    private WorkflowEnvironment environment = null!;

    public async Task InitializeAsync() =>
        environment = await WorkflowEnvironment.StartLocalAsync();

    public async Task DisposeAsync() => await environment.DisposeAsync();

    public enum Outcome { NoWorkflow, ExecuteFailure, ResultFailure, HistoryFailure, Success }
    public enum WorkerLifecycle { Running, Stopped, Restarted, Faulted }

    [Theory]
    [InlineData(Outcome.NoWorkflow, WorkerLifecycle.Running)]
    [InlineData(Outcome.ExecuteFailure, WorkerLifecycle.Running)]
    [InlineData(Outcome.ResultFailure, WorkerLifecycle.Running)]
    [InlineData(Outcome.HistoryFailure, WorkerLifecycle.Running)]
    [InlineData(Outcome.Success, WorkerLifecycle.Running)]
    [InlineData(Outcome.NoWorkflow, WorkerLifecycle.Stopped)]
    [InlineData(Outcome.NoWorkflow, WorkerLifecycle.Restarted)]
    [InlineData(Outcome.ExecuteFailure, WorkerLifecycle.Faulted)]
    [InlineData(Outcome.Success, WorkerLifecycle.Faulted)]
    public async Task RunAsyncStopsWorker(Outcome outcome, WorkerLifecycle workerLifecycle)
    {
        var runner = new Runner(
            new(environment.Client.Connection.Options.TargetHost!)
            {
                Namespace = environment.Client.Options.Namespace,
            },
            Guid.NewGuid().ToString(),
            null,
            new(typeof(TestFeature)),
            NullLoggerFactory.Instance,
            null);
        var feature = (TestFeature)runner.Feature;
        feature.Outcome = outcome;
        feature.WorkerLifecycle = workerLifecycle;

        try
        {
            var error = await Record.ExceptionAsync(() => runner.RunAsync(CancellationToken.None));
            if (outcome is Outcome.ExecuteFailure or Outcome.ResultFailure or Outcome.HistoryFailure)
            {
                Assert.Same(feature.Failure, error);
            }
            else if (workerLifecycle == WorkerLifecycle.Faulted)
            {
                Assert.Same(feature.WorkerFailure, error);
            }
            else
            {
                Assert.Null(error);
            }

            if (workerLifecycle == WorkerLifecycle.Faulted)
            {
                try
                {
                    await feature.WorkerTask.WaitAsync(TimeSpan.FromSeconds(5));
                }
                catch (OperationCanceledException)
                {
                }
            }
            else
            {
                Assert.True(feature.WorkerTask.IsCompleted, "Feature worker is still running");
            }
            Assert.Equal(outcome != Outcome.NoWorkflow && outcome != Outcome.ExecuteFailure, feature.ResultChecked);
            Assert.Equal(outcome is Outcome.HistoryFailure or Outcome.Success, feature.HistoryChecked);

            if (workerLifecycle != WorkerLifecycle.Faulted)
            {
                runner.StartWorker();
            }
        }
        finally
        {
            try
            {
                await runner.StopWorker();
            }
            catch (Exception e) when (e == feature.WorkerFailure)
            {
            }
        }
    }

    private class TestFeature : IFeature
    {
        public Outcome Outcome { get; set; }
        public WorkerLifecycle WorkerLifecycle { get; set; }
        public Exception Failure { get; } = new InvalidOperationException("Feature failed");
        public Exception WorkerFailure { get; } = new InvalidOperationException("Worker failed");
        public Task WorkerTask { get; private set; } = null!;
        public bool ResultChecked { get; private set; }
        public bool HistoryChecked { get; private set; }

        public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
            options.AddAllActivities(this);

        [Activity]
        public string Echo(string value) => value;

        public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
        {
            if (WorkerLifecycle is WorkerLifecycle.Stopped or WorkerLifecycle.Restarted)
            {
                await runner.StopWorker();
            }
            if (WorkerLifecycle == WorkerLifecycle.Restarted)
            {
                runner.StartWorker();
            }

            // Capture the task so clearing runner state alone cannot satisfy the cleanup assertion.
            var state = typeof(Runner).GetField("workerState", BindingFlags.Instance | BindingFlags.NonPublic)!
                .GetValue(runner);
            WorkerTask = state == null ? Task.CompletedTask :
                (Task)state.GetType().GetField("workerTask", BindingFlags.Instance | BindingFlags.NonPublic)!
                    .GetValue(state)!;
            if (WorkerLifecycle == WorkerLifecycle.Faulted)
            {
                // Inject a worker failure without depending on a particular SDK poller failure mode.
                state!.GetType().GetField("workerTask", BindingFlags.Instance | BindingFlags.NonPublic)!
                    .SetValue(state, Task.FromException(WorkerFailure));
            }

            if (Outcome == Outcome.ExecuteFailure)
            {
                throw Failure;
            }
            return Outcome == Outcome.NoWorkflow ? null : runner.Client.GetWorkflowHandle("unused");
        }

        public Task CheckResultAsync(Runner runner, WorkflowHandle handle)
        {
            ResultChecked = true;
            return Outcome == Outcome.ResultFailure ? Task.FromException(Failure) : Task.CompletedTask;
        }

        public Task CheckHistoryAsync(Runner runner, WorkflowHandle handle)
        {
            HistoryChecked = true;
            return Outcome == Outcome.HistoryFailure ? Task.FromException(Failure) : Task.CompletedTask;
        }
    }
}
