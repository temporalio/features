package sync_success_codec

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/features/data_converter/codec"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

var operation = nexus.NewSyncOperation(
	"run",
	func(ctx context.Context, input string, options nexus.StartOperationOptions) (string, error) {
		return "result", nil
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
	var result string
	err := future.Get(ctx, &result)
	return result, err
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   service,
	ExpectRunResult: "result",
	ClientOptions: client.Options{
		DataConverter: converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), codec.NewBase64Codec()),
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
			if a := event.GetNexusOperationCompletedEventAttributes(); a != nil {
				runner.Require.Equal("my-encoding", string(a.Result.GetMetadata()[converter.MetadataEncoding]))
			}
		}
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED)
	},
}
