package sync_unexpected_handler_error

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

// Count handler calls only; workflow execution and replay never read this state.
var attempts atomic.Int32

var RetryingOperation = nexus.NewSyncOperation(
	"retry",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		if attempts.Add(1) == 1 {
			// Deliberately neither a Nexus handler error nor an operation error.
			return "", errors.New("unexpected implementation failure")
		}
		return "recovered", nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(RetryingOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	fut := workflow.NewNexusClient(endpoint, ServiceName).ExecuteOperation(ctx, RetryingOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	var result string
	err := fut.Get(ctx, &result)
	return result, err
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "recovered",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		attempts.Store(0)
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckResult: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		if err := runner.CheckResultDefault(ctx, run); err != nil {
			return err
		}
		if actual := attempts.Load(); actual != 2 {
			return harness.AppErrorf("expected one failed handler call and one successful retry, got %d calls", actual)
		}
		return nil
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED)
	},
}
