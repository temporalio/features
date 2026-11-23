package operation_not_found

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Service = func() *nexus.Service {
	s := nexus.NewService("test-service")
	s.MustRegister(nexus.NewSyncOperation(
		"echo",
		func(ctx context.Context, input string, options nexus.StartOperationOptions) (string, error) {
			return input, nil
		},
	))
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	// Invoke by name so lookup happens on the worker, not in a local operation registry.
	fut := workflow.NewNexusClient(endpoint, "test-service").ExecuteOperation(ctx, "missing-operation", "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, "test-service", "missing-operation", "")
	if err != nil {
		return "", err
	}
	handlerErr, ok := opErr.Unwrap().(*nexus.HandlerError)
	if !ok || handlerErr.Type != nexus.HandlerErrorTypeNotFound ||
		handlerErr.RetryBehavior != nexus.HandlerErrorRetryBehaviorUnspecified || handlerErr.Retryable() {
		return "", harness.AppErrorf("expected default non-retryable NOT_FOUND handler cause, got %T: %v", opErr.Unwrap(), opErr.Unwrap())
	}
	if !strings.Contains(handlerErr.Message, "missing-operation") {
		return "", harness.AppErrorf("handler diagnostic does not identify missing operation: %q", handlerErr.Message)
	}
	return "operation not found", nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "operation not found",
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
