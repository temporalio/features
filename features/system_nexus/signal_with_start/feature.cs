namespace system_nexus.signal_with_start;

using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    [Workflow]
    class TargetWorkflow
    {
        private readonly List<string> signals = new();

        [WorkflowRun]
        public async Task<IReadOnlyCollection<string>> RunAsync(string value)
        {
            await Workflow.WaitConditionAsync(() => signals.Count == 2);
            return [$"started: {value}", .. signals];
        }

        [WorkflowSignal("add")]
        public Task AddAsync(string value)
        {
            signals.Add($"signal: {value}");
            return Task.CompletedTask;
        }
    }

    [Workflow]
    class CallerWorkflow
    {
        [WorkflowRun]
        public async Task<string[]> RunAsync(
            string targetId,
            string taskQueue)
        {
            var firstHandle = await Workflow.SignalWithStartWorkflowAsync(
                (TargetWorkflow workflow) => workflow.RunAsync("start-value"),
                workflow => workflow.AddAsync("signal-one"),
                new(targetId, taskQueue)
                {
                    ExecutionTimeout = TimeSpan.FromMinutes(1),
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });
            var secondHandle = await Workflow.SignalWithStartWorkflowAsync(
                (TargetWorkflow workflow) => workflow.RunAsync("unused-start-value"),
                workflow => workflow.AddAsync("signal-two"),
                new(targetId, taskQueue)
                {
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });

            var firstRunId = firstHandle.RunId;
            var secondRunId = secondHandle.RunId;
            if (firstRunId is null)
            {
                throw new ApplicationFailureException(
                    "first SignalWithStart returned no run ID",
                    nonRetryable: true);
            }

            if (secondRunId is null)
            {
                throw new ApplicationFailureException(
                    "second SignalWithStart returned no run ID",
                    nonRetryable: true);
            }

            if (firstRunId != secondRunId)
            {
                throw new ApplicationFailureException(
                    $"SignalWithStart calls targeted different runs: first={firstRunId} second={secondRunId}",
                    nonRetryable: true);
            }

            return [targetId, firstRunId];
        }
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options
            .AddWorkflow<CallerWorkflow>()
            .AddWorkflow<TargetWorkflow>();

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var taskQueue = runner.WorkerOptions.TaskQueue!;
        var targetId = $"system-nexus-signal-with-start-{taskQueue}";
        return await runner.Client.StartWorkflowAsync(
            (CallerWorkflow workflow) => workflow.RunAsync(targetId, taskQueue),
            runner.NewWorkflowOptions());
    }

    public async Task CheckResultAsync(Runner runner, WorkflowHandle handle)
    {
        var callerResult = await handle.GetResultAsync<string[]>();
        var targetId = callerResult[0];
        var targetRunId = callerResult[1];
        var target = runner.Client.GetWorkflowHandle<
            TargetWorkflow,
            IReadOnlyCollection<string>>(targetId, targetRunId);
        Assert.Equal(
            new[] { "started: start-value", "signal: signal-one", "signal: signal-two" },
            await target.GetResultAsync());
    }
}
