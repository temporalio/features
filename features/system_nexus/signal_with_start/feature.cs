namespace system_nexus.signal_with_start;

using System.Collections.Concurrent;
using System.Text.Json;
using Google.Protobuf;
using Temporalio.Api.Common.V1;
using Temporalio.Api.Enums.V1;
using Temporalio.Client;
using Temporalio.Converters;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    private static readonly ConcurrentQueue<SerializationRecord> SerializationRecords = new();

    private record ContextValue(string Label);

    private record SerializationRecord(string Stage, string Label, string WorkflowId);

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

        [WorkflowSignal]
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
        public async Task<string> RunAsync(string targetId, string taskQueue)
        {
            await Workflow.SignalWithStartWorkflowAsync(
                (TargetWorkflow workflow) => workflow.RunAsync("start-value"),
                workflow => workflow.AddAsync("signal-one"),
                new(targetId, taskQueue)
                {
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });
            await Workflow.SignalWithStartWorkflowAsync(
                (TargetWorkflow workflow) => workflow.RunAsync("unused-start-value"),
                workflow => workflow.AddAsync("signal-two"),
                new(targetId, taskQueue)
                {
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });
            return targetId;
        }
    }

    // This execution deliberately uses a distinct workflow pair and payload values. It verifies
    // the converter and codec are applied to the inner Signal-with-Start request payloads with
    // the target workflow's serialization context.
    [Workflow]
    class ContextTargetWorkflow
    {
        private readonly List<ContextValue> signals = new();

        [WorkflowRun]
        public async Task<IReadOnlyCollection<string>> RunAsync(ContextValue value)
        {
            await Workflow.WaitConditionAsync(() => signals.Count == 2);
            return [value.Label, .. signals.Select(signal => signal.Label)];
        }

        [WorkflowSignal]
        public Task AddAsync(ContextValue value)
        {
            signals.Add(value);
            return Task.CompletedTask;
        }
    }

    [Workflow]
    class ContextCallerWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync(string targetId, string taskQueue)
        {
            await Workflow.SignalWithStartWorkflowAsync(
                (ContextTargetWorkflow workflow) => workflow.RunAsync(new("context-start")),
                workflow => workflow.AddAsync(new("context-signal-one")),
                new(targetId, taskQueue)
                {
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });
            await Workflow.SignalWithStartWorkflowAsync(
                (ContextTargetWorkflow workflow) => workflow.RunAsync(new("unused-context-start")),
                workflow => workflow.AddAsync(new("context-signal-two")),
                new(targetId, taskQueue)
                {
                    IdConflictPolicy = WorkflowIdConflictPolicy.UseExisting,
                });
            return targetId;
        }
    }

    class ContextJsonPlainConverter : JsonPlainConverter, IWithSerializationContext<IEncodingConverter>
    {
        private readonly string? workflowId;

        public ContextJsonPlainConverter(string? workflowId = null)
            : base(new()) => this.workflowId = workflowId;

        public IEncodingConverter WithSerializationContext(ISerializationContext context) =>
            new ContextJsonPlainConverter(WorkflowIdFor(context));

        public override bool TryToPayload(object? value, out Payload? payload)
        {
            var converted = base.TryToPayload(value, out payload);
            if (converted && workflowId != null && value is ContextValue contextValue)
            {
                SerializationRecords.Enqueue(new("converter-encode", contextValue.Label, workflowId));
            }
            return converted;
        }

        public override object? ToValue(Payload payload, Type type)
        {
            var value = base.ToValue(payload, type);
            if (workflowId != null && value is ContextValue contextValue)
            {
                SerializationRecords.Enqueue(new("converter-decode", contextValue.Label, workflowId));
            }
            return value;
        }
    }

    class ContextPayloadCodec : IPayloadCodec, IWithSerializationContext<IPayloadCodec>
    {
        private readonly string? workflowId;

        public ContextPayloadCodec(string? workflowId = null) => this.workflowId = workflowId;

        public IPayloadCodec WithSerializationContext(ISerializationContext context) =>
            new ContextPayloadCodec(WorkflowIdFor(context));

        public Task<IReadOnlyCollection<Payload>> EncodeAsync(IReadOnlyCollection<Payload> payloads)
        {
            Record("codec-encode", payloads);
            return Task.FromResult(payloads);
        }

        public Task<IReadOnlyCollection<Payload>> DecodeAsync(IReadOnlyCollection<Payload> payloads)
        {
            Record("codec-decode", payloads);
            return Task.FromResult(payloads);
        }

        private void Record(string stage, IReadOnlyCollection<Payload> payloads)
        {
            if (workflowId == null)
            {
                return;
            }
            foreach (var payload in payloads)
            {
                // System Nexus envelopes must be traversed, not passed to application codecs.
                if (payload.Metadata.TryGetValue("messageType", out var messageType) &&
                    messageType.ToStringUtf8().StartsWith("temporal.api.workflowservice.v1.SignalWithStart"))
                {
                    throw new InvalidOperationException("Codec received a Signal-with-Start envelope");
                }
                if (!payload.Metadata.TryGetValue("encoding", out var encoding) ||
                    encoding.ToStringUtf8() != "json/plain")
                {
                    continue;
                }
                try
                {
                    var value = JsonSerializer.Deserialize<ContextValue>(payload.Data.Span);
                    if (value != null && value.Label.StartsWith("context-"))
                    {
                        SerializationRecords.Enqueue(new(stage, value.Label, workflowId));
                    }
                }
                catch (JsonException)
                {
                    // This is an unrelated json/plain payload.
                }
            }
        }
    }

    private static string WorkflowIdFor(ISerializationContext context) =>
        ((ISerializationContext.IHasWorkflow)context).WorkflowId ??
        throw new InvalidOperationException("Expected a workflow serialization context");

    public void ConfigureClient(Runner runner, TemporalClientConnectOptions options)
    {
        var defaultConverters = ((DefaultPayloadConverter)DataConverter.Default.PayloadConverter)
            .EncodingConverters
            .Where(converter => converter is not JsonPlainConverter);
        options.DataConverter = DataConverter.Default with
        {
            PayloadConverter = new DefaultPayloadConverter(
                [new ContextJsonPlainConverter(), .. defaultConverters]),
            PayloadCodec = new ContextPayloadCodec(),
        };
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options
            .AddWorkflow<CallerWorkflow>()
            .AddWorkflow<TargetWorkflow>()
            .AddWorkflow<ContextCallerWorkflow>()
            .AddWorkflow<ContextTargetWorkflow>();

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var targetId = $"{runner.PreparedFeature.Dir}-target";
        return await runner.Client.StartWorkflowAsync(
            (CallerWorkflow workflow) => workflow.RunAsync(targetId, runner.WorkerOptions.TaskQueue!),
            runner.NewWorkflowOptions());
    }

    public async Task CheckResultAsync(Runner runner, WorkflowHandle handle)
    {
        var targetId = await handle.GetResultAsync<string>();
        var target = runner.Client.GetWorkflowHandle<TargetWorkflow, IReadOnlyCollection<string>>(targetId);
        Assert.Equal(
            new[] { "started: start-value", "signal: signal-one", "signal: signal-two" },
            await target.GetResultAsync());

        while (SerializationRecords.TryDequeue(out _))
        {
        }
        var contextTargetId = $"{runner.PreparedFeature.Dir}-context-target";
        var contextCaller = await runner.Client.StartWorkflowAsync(
            (ContextCallerWorkflow workflow) =>
                workflow.RunAsync(contextTargetId, runner.WorkerOptions.TaskQueue!),
            runner.NewWorkflowOptions());
        Assert.Equal(contextTargetId, await contextCaller.GetResultAsync<string>());
        var contextTarget = runner.Client.GetWorkflowHandle<ContextTargetWorkflow, IReadOnlyCollection<string>>(
            contextTargetId);
        Assert.Equal(
            new[] { "context-start", "context-signal-one", "context-signal-two" },
            await contextTarget.GetResultAsync());

        var records = SerializationRecords.ToArray();
        foreach (var stage in new[]
        {
            "converter-encode",
            "converter-decode",
            "codec-encode",
            "codec-decode",
        })
        {
            var stageRecords = records.Where(record => record.Stage == stage).ToArray();
            Assert.True(stageRecords.Length >= 3, $"Expected three {stage} records");
            Assert.All(stageRecords, record => Assert.Equal(contextTargetId, record.WorkflowId));
            foreach (var label in new[] { "context-start", "context-signal-one", "context-signal-two" })
            {
                Assert.Contains(stageRecords, record => record.Label == label);
            }
        }
    }
}
