namespace tracing.encoded_headers;

using System.Diagnostics;
using System.Text.Json;
using Google.Protobuf;
using OpenTelemetry.Context.Propagation;
using Temporalio.Api.Common.V1;
using Temporalio.Client;
using Temporalio.Converters;
using Temporalio.Extensions.OpenTelemetry;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

public class Feature : IFeature
{
    private static readonly ActivityListener Listener;

    static Feature()
    {
        var clientSourceName = TracingInterceptor.ClientSource.Name;
        var workflowSourceName = TracingInterceptor.WorkflowsSource.Name;
        Listener = new()
        {
            ShouldListenTo = source => source.Name == "header-codec-feature" ||
                source.Name == clientSourceName || source.Name == workflowSourceName,
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
        };
        ActivitySource.AddActivityListener(Listener);
    }

    public void ConfigureClient(Runner runner, TemporalClientConnectOptions options)
    {
        options.DataConverter = DataConverter.Default with { PayloadCodec = new WrappingCodec() };
        options.Interceptors = new[]
        {
            new TracingInterceptor(new() { Propagator = new TraceContextPropagator() }),
        };
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<WorkflowImpl>();

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        using var source = new ActivitySource("header-codec-feature");
        using var parent = source.StartActivity("dotnet-caller")!;
        var options = runner.NewWorkflowOptions();
        options.ExecutionTimeout = TimeSpan.FromSeconds(15);
        var handle = await runner.Client.StartWorkflowAsync("workflow", new object?[] { "hello" }, options);
        try
        {
            var history = await handle.FetchHistoryAsync();
            var header = history.Events.First().WorkflowExecutionStartedEventAttributes.Header.Fields["_tracer-data"];
            Assert.Equal(WrappingCodec.EncodingName, header.Metadata["encoding"].ToStringUtf8());
            var decoded = (await new WrappingCodec().DecodeAsync(new[] { header })).Single();
            var carrier = JsonSerializer.Deserialize<Dictionary<string, string>>(decoded.Data.ToStringUtf8())!;
            Assert.Equal(parent.TraceId.ToHexString(), carrier["traceparent"].Split('-')[1]);

            var result = await handle.GetResultAsync<Dictionary<string, string>>().WaitAsync(TimeSpan.FromSeconds(30));
            Assert.Equal("hello", result["message"]);
            Assert.Equal(parent.TraceId.ToHexString(), result["traceId"]);
            history = await handle.FetchHistoryAsync();
            Assert.DoesNotContain(history.Events, evt => evt.WorkflowTaskFailedEventAttributes != null);
            return null;
        }
        catch
        {
            // Preserve the extraction error in diagnostics instead of reporting only an execution timeout.
            foreach (var evt in (await handle.FetchHistoryAsync()).Events)
            {
                if (evt.WorkflowTaskFailedEventAttributes is { } failed)
                {
                    runner.Logger.LogError("Workflow Task failed: {Failure}", failed.Failure);
                }
            }
            throw;
        }
    }

    [Workflow("workflow")]
    public class WorkflowImpl
    {
        [WorkflowRun]
        public Task<Dictionary<string, string>> RunAsync(string message)
        {
            var activity = WorkflowDiagnosticActivity.Current ??
                throw new InvalidOperationException("No active workflow tracing span");
            Assert.NotEqual(default, activity.Context.TraceId);
            return Task.FromResult(new Dictionary<string, string>
            {
                ["message"] = message,
                ["traceId"] = activity.Context.TraceId.ToHexString(),
            });
        }
    }

    sealed class WrappingCodec : IPayloadCodec
    {
        public const string EncodingName = "binary/encrypted";

        public Task<IReadOnlyCollection<Payload>> EncodeAsync(IReadOnlyCollection<Payload> payloads) =>
            Task.FromResult<IReadOnlyCollection<Payload>>(payloads.Select(payload =>
            {
                Assert.NotEqual(EncodingName, payload.Metadata["encoding"].ToStringUtf8());
                return new Payload
                {
                    Metadata = { ["encoding"] = ByteString.CopyFromUtf8(EncodingName) },
                    Data = ByteString.CopyFrom(payload.ToByteArray().Select(b => (byte)(b ^ 0x5a)).ToArray()),
                };
            }).ToArray());

        public Task<IReadOnlyCollection<Payload>> DecodeAsync(IReadOnlyCollection<Payload> payloads) =>
            Task.FromResult<IReadOnlyCollection<Payload>>(payloads.Select(payload =>
            {
                Assert.Equal(EncodingName, payload.Metadata["encoding"].ToStringUtf8());
                return Payload.Parser.ParseFrom(payload.Data.Select(b => (byte)(b ^ 0x5a)).ToArray());
            }).ToArray());
    }
}
