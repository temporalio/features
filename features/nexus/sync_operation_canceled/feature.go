package sync_operation_canceled

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

var CanceledOperation = nexus.NewSyncOperation(
	"cancel",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		return "", &nexus.OperationError{
			State:   nexus.OperationStateCanceled,
			Message: "operation canceled by handler",
			Cause:   temporal.NewApplicationError("cancellation cause", "CancellationCause", "cancellation detail"),
		}
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(CanceledOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	fut := workflow.NewNexusClient(endpoint, ServiceName).ExecuteOperation(ctx, CanceledOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, ServiceName, "cancel", "")
	if err != nil {
		return "", err
	}
	canceledErr, ok := opErr.Unwrap().(*temporal.CanceledError)
	if !ok || canceledErr.Error() != "operation canceled by handler" || canceledErr.HasDetails() {
		return "", harness.AppErrorf("expected canceled cause with handler message and no details, got %v", opErr.Unwrap())
	}
	if err := harness.CheckNexusApplicationCause(canceledErr.Unwrap(), "CancellationCause", "cancellation cause", "cancellation detail"); err != nil {
		return "", err
	}
	return "canceled by handler", nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "canceled by handler",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		// There is no caller cancellation request, and a sync operation never starts.
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED)
	},
}
