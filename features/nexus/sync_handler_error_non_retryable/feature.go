package sync_handler_error_non_retryable

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

var FailingOperation = nexus.NewSyncOperation(
	"fail",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		// UNAVAILABLE is normally retryable. Explicitly overriding it exercises
		// retry-behavior translation independently of the handler error type.
		return "", &nexus.HandlerError{
			Type:          nexus.HandlerErrorTypeUnavailable,
			Message:       "handler rejected request",
			Cause:         temporal.NewApplicationError("handler cause", "HandlerCause", "handler detail"),
			RetryBehavior: nexus.HandlerErrorRetryBehaviorNonRetryable,
		}
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(FailingOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	fut := workflow.NewNexusClient(endpoint, ServiceName).ExecuteOperation(ctx, FailingOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, ServiceName, "fail", "")
	if err != nil {
		return "", err
	}
	handlerErr, ok := opErr.Unwrap().(*nexus.HandlerError)
	if !ok || handlerErr.Type != nexus.HandlerErrorTypeUnavailable ||
		handlerErr.RetryBehavior != nexus.HandlerErrorRetryBehaviorNonRetryable || handlerErr.Retryable() ||
		handlerErr.Message != "handler rejected request" {
		return "", harness.AppErrorf("expected non-retryable UNAVAILABLE handler cause, got %T: %v", opErr.Unwrap(), opErr.Unwrap())
	}
	if err := harness.CheckNexusApplicationCause(handlerErr.Unwrap(), "HandlerCause", "handler cause", "handler detail"); err != nil {
		return "", err
	}
	return "handler failed as expected", nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "handler failed as expected",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED)
	},
}
