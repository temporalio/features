package async_cancellation_try_cancel

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

func HandlerWorkflow(ctx workflow.Context, id string) (string, error) {
	disconnected, _ := workflow.NewDisconnectedContext(ctx)
	workflow.GetSignalChannel(disconnected, "release").Receive(disconnected, nil)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "released", nil
}

var Service = func() *nexus.Service {
	s := nexus.NewService("test-service")
	s.MustRegister(temporalnexus.NewWorkflowRunOperation(
		"block", HandlerWorkflow,
		func(ctx context.Context, id string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{ID: id}, nil
		}))
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, "test-service")
	id := workflow.GetInfo(ctx).WorkflowExecution.ID + "-handler"
	opCtx, cancel := workflow.WithCancel(ctx)
	fut := nc.ExecuteOperation(opCtx, "block", id, workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
		CancellationType:       workflow.NexusOperationCancellationTypeTryCancel,
	})
	var execution workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &execution); err != nil {
		return "", err
	}
	if execution.OperationToken == "" {
		return "", harness.AppErrorf("expected non-empty operation token")
	}
	cancel()
	err := fut.Get(ctx, nil)
	canceled, ok := err.(*temporal.CanceledError)
	if !ok || canceled.Unwrap() != nil || canceled.HasDetails() {
		return "", harness.AppErrorf("expected leaf local cancellation, got %T: %v", err, err)
	}
	return "canceled", nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, HandlerWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "canceled",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckResult: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		if err := runner.CheckResultDefault(ctx, run); err != nil {
			return err
		}
		id := run.GetID() + "-handler"
		if err := runner.Client.SignalWorkflow(ctx, id, "", "release", nil); err != nil {
			return err
		}
		var result string
		err := runner.Client.GetWorkflow(ctx, id, "").Get(ctx, &result)
		// TRY_CANCEL does not guarantee delivery after the caller exits.
		if temporal.IsCanceledError(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if result != "released" {
			return harness.AppErrorf("unexpected backing workflow result: %q", result)
		}
		return nil
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED)
	},
}
