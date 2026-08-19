package async_cancellation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
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
	})
	var exec workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &exec); err != nil {
		return "", err
	}
	cancel()

	err := fut.Get(ctx, nil)
	if err == nil {
		return "", harness.AppErrorf("expected the cancelled operation to fail")
	}
	var canceledErr *temporal.CanceledError
	if !errors.As(err, &canceledErr) {
		return "", harness.AppErrorf("expected a canceled error, got %v", err)
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
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		ev, err := harness.FindEvent(hist, func(ev *historypb.HistoryEvent) bool {
			return ev.EventType == enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED
		})
		if err != nil {
			return err
		}
		if ev == nil {
			return fmt.Errorf("did not find NexusOperationCancelRequested event in history")
		}
		return nil
	},
}
