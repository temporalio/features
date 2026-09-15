namespace data_converter.transfer_types;

using System.Text.Json;
using NexusRpc;
using NexusRpc.Handlers;
using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Converters;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;
using ApiWorkflowExecution = Temporalio.Api.Common.V1.WorkflowExecution;
using Payload = Temporalio.Api.Common.V1.Payload;
using TemporalRetryPolicy = Temporalio.Common.RetryPolicy;

class Feature : IFeature
{
    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<TransferWorkflow>().
            AddWorkflow<ThrowingWorkflow>().
            AddAllActivities(new TransferActivities()).
            AddNexusService(new TransferServiceHandler());

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        var failedWorkflowId = $"{runner.PreparedFeature.Dir}-failing-{Guid.NewGuid()}";
        var failedOptions = runner.NewWorkflowOptions();
        failedOptions.Id = failedWorkflowId;
        var exception = await Assert.ThrowsAsync<TransferConversionException>(() =>
            runner.Client.StartWorkflowAsync(
                (ThrowingWorkflow wf) => wf.RunAsync(
                    new ThrowingValue("expected transfer conversion failure")),
                failedOptions));
        Assert.Equal("expected transfer conversion failure", exception.Message);

        var rpcException = await Assert.ThrowsAsync<RpcException>(() =>
            runner.Client.GetWorkflowHandle(failedWorkflowId).DescribeAsync());
        Assert.Equal(RpcException.StatusCode.NotFound, rpcException.Code);

        await ExecuteStandaloneActivityAsync(runner);
        await ExecuteStandaloneNexusAsync(runner);

        return await runner.Client.StartWorkflowAsync(
            (TransferWorkflow wf) => wf.RunAsync(
                new NonGenericValue("non-generic", "client-extra"),
                new Box<int>(123, "client-extra"),
                new DerivedFromConvertedBase(
                    "converted-base", "client-extra", "client-derived-extra"),
                new DerivedFromConvertedBase(
                    "unconverted-derived", "plain-extra", "derived-extra"),
                new PlainValue(
                    "plain",
                    "plain-extra",
                    new NonGenericValue("nested", "nested-extra")),
                new ConvertedDerived(
                    "plain-base", "plain-extra", "ignored-derived-extra"),
                new ConvertedDerived(
                    "converted-derived", "client-extra", "client-derived-extra")),
            runner.NewWorkflowOptions());
    }

    public async Task CheckResultAsync(Runner runner, WorkflowHandle handle)
    {
        var result = await handle.GetResultAsync<NonGenericValue>();
        Assert.True(
            result == new NonGenericValue(
                "workflow-result", TransferModels.TransferredMarker),
            result.Value);

        var history = await handle.FetchHistoryAsync();
        var started = history.Events.Single(
            evt => evt.WorkflowExecutionStartedEventAttributes != null).
            WorkflowExecutionStartedEventAttributes;
        var inputs = started.Input.Payloads_;
        Assert.Equal(7, inputs.Count);
        AssertProtobufPayload(inputs[0], "non-generic");
        AssertProtobufPayload(inputs[1], "box");
        AssertJsonPayload(inputs[2], TransferModels.TransferredMarker);
        AssertJsonPayload(inputs[3]);
        AssertJsonPayload(inputs[4]);
        AssertJsonPayload(inputs[5], "plain-extra");
        AssertJsonPayload(inputs[6], TransferModels.TransferredMarker);

        var activityFailed = history.Events.Single(
            evt => evt.ActivityTaskFailedEventAttributes != null).
            ActivityTaskFailedEventAttributes;
        var detailPayload = activityFailed.Failure.ApplicationFailureInfo.Details.Payloads_.Single();
        AssertProtobufPayload(detailPayload, "non-generic");

        var completed = history.Events.Single(
            evt => evt.WorkflowExecutionCompletedEventAttributes != null).
            WorkflowExecutionCompletedEventAttributes;
        AssertProtobufPayload(completed.Result.Payloads_.Single(), "non-generic");
    }

    private static async Task ExecuteStandaloneActivityAsync(Runner runner)
    {
        var result = await runner.Client.ExecuteActivityAsync(
            () => TransferActivities.TransferAsync(
                new NonGenericValue("activity-input", "client-extra")),
            new StartActivityOptions(
                $"{runner.PreparedFeature.Dir}-activity-{Guid.NewGuid()}",
                runner.WorkerOptions.TaskQueue!)
            {
                StartToCloseTimeout = TimeSpan.FromSeconds(10),
                RetryPolicy = new TemporalRetryPolicy { MaximumAttempts = 1 },
            });
        Assert.Equal(
            new NonGenericValue("activity-result", TransferModels.TransferredMarker),
            result);
    }

    private static async Task ExecuteStandaloneNexusAsync(Runner runner)
    {
        if (runner.NexusEndpoint == null)
        {
            runner.Logger.LogInformation(
                "Skipping Standalone Nexus check because no endpoint is available");
            return;
        }

        var client = runner.Client.CreateNexusClient<ITransferService>(runner.NexusEndpoint);
        try
        {
            var result = await client.ExecuteNexusOperationAsync(
                service => service.Transfer(
                    new NonGenericValue("nexus-input", "client-extra")),
                new($"{runner.PreparedFeature.Dir}-nexus-{Guid.NewGuid()}")
                {
                    ScheduleToCloseTimeout = TimeSpan.FromSeconds(10),
                });
            Assert.Equal(
                new NonGenericValue("nexus-result", TransferModels.TransferredMarker),
                result);
        }
        catch (RpcException e) when (e.Code == RpcException.StatusCode.Unimplemented)
        {
            runner.Logger.LogInformation(
                "Skipping Standalone Nexus check because the server does not support it");
        }
    }

    private static void AssertProtobufPayload(Payload payload, string expectedRunId)
    {
        Assert.Equal("json/protobuf", payload.Metadata["encoding"].ToStringUtf8());
        Assert.Equal(
            "temporal.api.common.v1.WorkflowExecution",
            payload.Metadata["messageType"].ToStringUtf8());
        var value = Assert.IsType<ApiWorkflowExecution>(
            new JsonProtoConverter().ToValue(payload, typeof(ApiWorkflowExecution)));
        Assert.Equal(expectedRunId, value.RunId);
    }

    private static void AssertJsonPayload(Payload payload, string? expectedExtra = null)
    {
        Assert.Equal("json/plain", payload.Metadata["encoding"].ToStringUtf8());
        if (expectedExtra != null)
        {
            using var json = JsonDocument.Parse(payload.Data.ToStringUtf8());
            Assert.Equal(expectedExtra, json.RootElement.GetProperty("Extra").GetString());
        }
    }

    [Workflow]
    class TransferWorkflow
    {
        [WorkflowRun]
        public async Task<NonGenericValue> RunAsync(
            NonGenericValue nonGeneric,
            Box<int> box,
            ConvertedBase convertedBase,
            DerivedFromConvertedBase unconvertedDerived,
            PlainValue plain,
            PlainBase plainBase,
            ConvertedDerived convertedDerived)
        {
            var failures = new List<string>();
            Check(
                nonGeneric == new NonGenericValue(
                    "non-generic", TransferModels.TransferredMarker),
                "non-generic");
            Check(box == new Box<int>(123, TransferModels.TransferredMarker), "generic");
            Check(convertedBase.GetType() == typeof(ConvertedBase), "base-type");
            Check(
                convertedBase == new ConvertedBase(
                    "converted-base", TransferModels.TransferredMarker),
                "base-converter");
            Check(
                unconvertedDerived.GetType() == typeof(DerivedFromConvertedBase),
                "exact-declaration-type");
            Check(
                unconvertedDerived == new DerivedFromConvertedBase(
                    "unconverted-derived", "plain-extra", "derived-extra"),
                "exact-declaration-value");
            Check(plainBase.GetType() == typeof(PlainBase), "declared-plain-base-type");
            Check(
                plainBase == new PlainBase("plain-base", "plain-extra"),
                "declared-plain-base-value");
            Check(
                convertedDerived == new ConvertedDerived(
                    "converted-derived",
                    TransferModels.TransferredMarker,
                    TransferModels.TransferredMarker),
                "converted-derived");
            Check(
                plain == new PlainValue(
                    "plain",
                    "plain-extra",
                    new NonGenericValue("nested", "nested-extra")),
                "top-level-only");

            try
            {
                await Workflow.ExecuteActivityAsync(
                    (TransferActivities act) => act.FailWithTransferDetailAsync(),
                    new ActivityOptions
                    {
                        StartToCloseTimeout = TimeSpan.FromSeconds(10),
                        RetryPolicy = new TemporalRetryPolicy { MaximumAttempts = 1 },
                    });
                failures.Add("failure-detail-missing");
            }
            catch (ActivityFailureException e)
            {
                if (e.InnerException is not ApplicationFailureException appFailure)
                {
                    failures.Add("failure-detail-error");
                }
                else
                {
                    Check(appFailure.Details.Count == 1, "failure-detail-count");
                    if (appFailure.Details.Count == 1)
                    {
                        var detail = appFailure.Details.ElementAt<ApiWorkflowExecution>(0);
                        Check(
                            detail.WorkflowId == "failure-detail" &&
                            detail.RunId == "non-generic",
                            "failure-detail-value");
                    }
                }
            }

            var resultValue = failures.Count == 0 ?
                "workflow-result" : $"failed:{string.Join(',', failures)}";
            return new NonGenericValue(resultValue, "must-not-be-serialized");

            void Check(bool condition, string name)
            {
                if (!condition)
                {
                    failures.Add(name);
                }
            }
        }
    }

    [Workflow]
    class ThrowingWorkflow
    {
        [WorkflowRun]
        public Task RunAsync(ThrowingValue value) =>
            throw new InvalidOperationException("A converter failure must prevent workflow start");
    }

    class TransferActivities
    {
        [Activity]
        public static Task<NonGenericValue> TransferAsync(NonGenericValue value)
        {
            Assert.Equal(
                new NonGenericValue("activity-input", TransferModels.TransferredMarker),
                value);
            return Task.FromResult(
                new NonGenericValue("activity-result", "must-not-be-serialized"));
        }

        [Activity]
        public Task FailWithTransferDetailAsync() =>
            throw new ApplicationFailureException(
                "intentional transfer detail failure",
                nonRetryable: true,
                details: new object?[]
                {
                    new NonGenericValue("failure-detail", "must-not-be-serialized"),
                });
    }

    [NexusService]
    interface ITransferService
    {
        [NexusOperation]
        NonGenericValue Transfer(NonGenericValue input);
    }

    [NexusServiceHandler(typeof(ITransferService))]
    class TransferServiceHandler
    {
        [NexusOperationHandler]
        public IOperationHandler<NonGenericValue, NonGenericValue> Transfer() =>
            OperationHandler.Sync<NonGenericValue, NonGenericValue>((context, value) =>
            {
                Assert.Equal(
                    new NonGenericValue(
                        "nexus-input", TransferModels.TransferredMarker),
                    value);
                return new NonGenericValue("nexus-result", "must-not-be-serialized");
            });
    }
}

static class TransferModels
{
    public const string TransferredMarker = "created-from-transfer-type";
}

[TemporalTransferTypeConverter(typeof(NonGenericValueConverter))]
record NonGenericValue(string Value, string Extra);

sealed class NonGenericValueConverter : ITemporalTransferTypeConverter
{
    public Type TransferType => typeof(ApiWorkflowExecution);

    public object ToTransferType(object? value) => new ApiWorkflowExecution
    {
        WorkflowId = ((NonGenericValue)value!).Value,
        RunId = "non-generic",
    };

    public object FromTransferType(object? transferType)
    {
        var value = (ApiWorkflowExecution)transferType!;
        Assert.Equal("non-generic", value.RunId);
        return new NonGenericValue(value.WorkflowId, TransferModels.TransferredMarker);
    }
}

[TemporalTransferTypeConverter(typeof(BoxConverter<>))]
record Box<T>(T Value, string Extra);

sealed class BoxConverter<T> : ITemporalTransferTypeConverter
{
    public Type TransferType => typeof(ApiWorkflowExecution);

    public object ToTransferType(object? value) => new ApiWorkflowExecution
    {
        WorkflowId = Convert.ToString(((Box<T>)value!).Value)!,
        RunId = "box",
    };

    public object FromTransferType(object? transferType)
    {
        var value = (ApiWorkflowExecution)transferType!;
        Assert.Equal("box", value.RunId);
        return new Box<T>(
            (T)Convert.ChangeType(value.WorkflowId, typeof(T)),
            TransferModels.TransferredMarker);
    }
}

[TemporalTransferTypeConverter(typeof(ConvertedBaseConverter))]
record ConvertedBase(string Value, string Extra);

record DerivedFromConvertedBase(string Value, string Extra, string DerivedExtra) :
    ConvertedBase(Value, Extra);

sealed class ConvertedBaseConverter : ITemporalTransferTypeConverter
{
    public Type TransferType => typeof(PlainBase);

    public object ToTransferType(object? value) =>
        new PlainBase(
            ((ConvertedBase)value!).Value,
            TransferModels.TransferredMarker);

    public object FromTransferType(object? transferType)
    {
        var value = (PlainBase)transferType!;
        return new ConvertedBase(value.Value, value.Extra);
    }
}

record PlainBase(string Value, string Extra);

[TemporalTransferTypeConverter(typeof(ConvertedDerivedConverter))]
record ConvertedDerived(string Value, string Extra, string DerivedExtra) :
    PlainBase(Value, Extra);

sealed class ConvertedDerivedConverter : ITemporalTransferTypeConverter
{
    public Type TransferType => typeof(PlainBase);

    public object ToTransferType(object? value)
    {
        var model = (ConvertedDerived)value!;
        return new PlainBase(model.Value, TransferModels.TransferredMarker);
    }

    public object FromTransferType(object? transferType)
    {
        var value = (PlainBase)transferType!;
        return new ConvertedDerived(
            value.Value,
            TransferModels.TransferredMarker,
            TransferModels.TransferredMarker);
    }
}

record PlainValue(string Value, string Extra, NonGenericValue Nested);

[TemporalTransferTypeConverter(typeof(ThrowingValueConverter))]
record ThrowingValue(string Value);

sealed class ThrowingValueConverter : ITemporalTransferTypeConverter
{
    public Type TransferType => typeof(ApiWorkflowExecution);

    public object? ToTransferType(object? value) =>
        throw new TransferConversionException(((ThrowingValue)value!).Value);

    public object? FromTransferType(object? transferType) =>
        throw new TransferConversionException(
            ((ApiWorkflowExecution)transferType!).WorkflowId);
}

sealed class TransferConversionException : Exception
{
    public TransferConversionException(string message)
        : base(message)
    {
    }
}
