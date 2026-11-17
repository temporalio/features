package async_cancellation

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

// BlockingWorkflow never completes on its own - it only ends when cancelled, which makes the
// cancellation race deterministic.
func BlockingWorkflow(ctx workflow.Context, name string) (string, error) {
	ctx.Done().Receive(ctx, nil)
	return "", ctx.Err()
}

var AsyncOperation = temporalnexus.NewWorkflowRunOperation(
	"block-forever",
	BlockingWorkflow,
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		return client.StartWorkflowOptions{ID: "async-cancellation-" + name}, nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(AsyncOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	opCtx, cancel := workflow.WithCancel(ctx)
	fut := nc.ExecuteOperation(opCtx, AsyncOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
		CancellationType:       workflow.NexusOperationCancellationTypeWaitCompleted,
	})
	var exec workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &exec); err != nil {
		return "", err
	}
	if exec.OperationToken == "" {
		return "", harness.AppErrorf("expected non-empty operation token")
	}
	cancel()

	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, ServiceName, "block-forever", exec.OperationToken)
	if err != nil {
		return "", err
	}
	canceledErr, ok := opErr.Unwrap().(*temporal.CanceledError)
	if !ok || canceledErr.Unwrap() != nil || canceledErr.HasDetails() {
		return "", harness.AppErrorf("expected immediate leaf canceled cause without details, got %T: %v", opErr.Unwrap(), opErr.Unwrap())
	}
	return "canceled", nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, BlockingWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "canceled",
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
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED)
	},
}
