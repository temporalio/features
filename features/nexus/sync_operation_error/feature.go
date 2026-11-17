package sync_operation_error

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

const (
	ServiceName  = "test-service"
	ErrorType    = "TestFailure"
	ErrorMessage = "deliberate failure"
	ErrorDetail  = "application detail"
)

var FailingOperation = nexus.NewSyncOperation(
	"fail",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		return "", &nexus.OperationError{
			State:   nexus.OperationStateFailed,
			Message: "operation failed",
			Cause:   temporal.NewApplicationError(ErrorMessage, ErrorType, ErrorDetail),
		}
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(FailingOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	fut := nc.ExecuteOperation(ctx, FailingOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	err := fut.Get(ctx, nil)
	if err == nil {
		return "", harness.AppErrorf("expected the operation to fail")
	}
	opErr, err := harness.CheckNexusOperationError(err, endpoint, ServiceName, "fail", "")
	if err != nil {
		return "", err
	}
	wrapper, ok := opErr.Unwrap().(*temporal.ApplicationError)
	if !ok || wrapper.Type() != "OperationError" || wrapper.Message() != "operation failed" {
		return "", harness.AppErrorf("expected OperationError wrapper, got %v", opErr.Unwrap())
	}
	if err := harness.CheckNexusApplicationCause(wrapper.Unwrap(), ErrorType, ErrorMessage, ErrorDetail); err != nil {
		return "", err
	}
	return ErrorType + ": " + ErrorMessage, nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: ErrorType + ": " + ErrorMessage,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED)
	},
}
