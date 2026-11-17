package sync_operation_error_codec

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/features/data_converter/codec"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var operation = nexus.NewSyncOperation(
	"run",
	func(ctx context.Context, input string, options nexus.StartOperationOptions) (string, error) {
		return "", &nexus.OperationError{
			State:   nexus.OperationStateFailed,
			Message: "operation failed",
			Cause:   temporal.NewApplicationError("deliberate failure", "CodecFailure", "detail"),
		}
	},
)

var service = func() *nexus.Service {
	s := nexus.NewService("codec-service")
	s.MustRegister(operation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	future := workflow.NewNexusClient(endpoint, "codec-service").ExecuteOperation(
		ctx, operation, "input", workflow.NexusOperationOptions{ScheduleToCloseTimeout: time.Minute})
	opErr, err := harness.CheckNexusOperationError(future.Get(ctx, nil), endpoint, "codec-service", "run", "")
	if err != nil {
		return "", err
	}
	wrapper, ok := opErr.Unwrap().(*temporal.ApplicationError)
	if !ok || wrapper.Type() != "OperationError" || wrapper.Message() != "operation failed" {
		return "", harness.AppErrorf("unexpected operation error: %v", opErr.Unwrap())
	}
	if err := harness.CheckNexusApplicationCause(wrapper.Unwrap(), "CodecFailure", "deliberate failure", "detail"); err != nil {
		return "", err
	}
	return "ok", nil
}

var dataConverter = converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), codec.NewBase64Codec())

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   service,
	ExpectRunResult: "ok",
	ClientOptions: client.Options{
		DataConverter: dataConverter,
		FailureConverter: temporal.NewDefaultFailureConverter(temporal.DefaultFailureConverterOptions{
			DataConverter:          dataConverter,
			EncodeCommonAttributes: true,
		}),
	},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for hist.HasNext() {
			event, err := hist.Next()
			if err != nil {
				return err
			}
			if a := event.GetNexusOperationScheduledEventAttributes(); a != nil {
				runner.Require.Equal("my-encoding", string(a.Input.GetMetadata()[converter.MetadataEncoding]))
			}
			if a := event.GetNexusOperationFailedEventAttributes(); a != nil {
				// NexusOperationFailure -> OperationError -> application cause.
				wrapper := a.Failure.GetCause()
				cause := wrapper.GetCause()
				for _, failure := range []*failurepb.Failure{wrapper, cause} {
					runner.Require.Equal("Encoded failure", failure.GetMessage())
					runner.Require.Empty(failure.GetStackTrace())
					runner.Require.Equal("my-encoding", string(failure.GetEncodedAttributes().GetMetadata()[converter.MetadataEncoding]))
				}
				details := cause.GetApplicationFailureInfo().GetDetails()
				runner.Require.Equal(1, len(details.GetPayloads()))
				runner.Require.Equal("my-encoding", string(details.Payloads[0].GetMetadata()[converter.MetadataEncoding]))
			}
		}
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED)
	},
}
