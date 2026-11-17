package async_cancellation_abandon

import (
	"context"
	"fmt"
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
	workflow.GetSignalChannel(ctx, "release").Receive(ctx, nil)
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
	s.MustRegister(nexus.NewSyncOperation(
		"release",
		func(ctx context.Context, id string, options nexus.StartOperationOptions) (string, error) {
			c := temporalnexus.GetClient(ctx)
			if err := c.SignalWorkflow(ctx, id, "", "release", nil); err != nil {
				return "", err
			}
			var result string
			err := c.GetWorkflow(ctx, id, "").Get(ctx, &result)
			return result, err
		}))
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, "test-service")
	id := workflow.GetInfo(ctx).WorkflowExecution.ID + "-handler"
	opCtx, cancel := workflow.WithCancel(ctx)
	fut := nc.ExecuteOperation(opCtx, "block", id, workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
		CancellationType:       workflow.NexusOperationCancellationTypeAbandon,
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
	var result string
	err = nc.ExecuteOperation(ctx, "release", id, workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	}).Get(ctx, &result)
	return result, err
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, HandlerWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "released",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		var releaseID int64
		scheduled, started, releaseCompleted := 0, 0, false
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for hist.HasNext() {
			event, err := hist.Next()
			if err != nil {
				return err
			}
			if attrs := event.GetNexusOperationScheduledEventAttributes(); attrs != nil {
				scheduled++
				if scheduled == 1 && attrs.Operation != "block" || scheduled == 2 && attrs.Operation != "release" {
					return fmt.Errorf("unexpected scheduled operation %s", attrs.Operation)
				}
				if attrs.Operation == "release" {
					releaseID = event.EventId
				}
			}
			switch event.EventType {
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED:
				started++
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED:
				if releaseID == 0 {
					return fmt.Errorf("operation completed before release")
				}
				if event.GetNexusOperationCompletedEventAttributes().ScheduledEventId == releaseID {
					releaseCompleted = true
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED, enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_COMPLETED,
				enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_FAILED, enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED,
				enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED, enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT:
				return fmt.Errorf("unexpected Nexus event for abandoned operation: %v", event.EventType)
			}
		}
		if scheduled != 2 || started != 1 || !releaseCompleted {
			return fmt.Errorf("unexpected lifecycle: scheduled=%d started=%d releaseCompleted=%t", scheduled, started, releaseCompleted)
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
