package async_cancellation_wait_requested

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

var allowCancellation = make(chan struct{})
var allowOnce sync.Once

func CancellationGate(ctx context.Context, release bool) error {
	if release {
		allowOnce.Do(func() { close(allowCancellation) })
	}
	return nil
}

type gatedCancellation struct {
	nexus.Operation[string, string]
}

func (o *gatedCancellation) Cancel(ctx context.Context, token string, options nexus.CancelOperationOptions) error {
	select {
	case <-allowCancellation:
		return o.Operation.Cancel(ctx, token, options)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Receiving cancellation does not finish this workflow. The caller must regain
// control and invoke release before the operation can complete.
func HandlerWorkflow(ctx workflow.Context, id string) (string, error) {
	disconnected, _ := workflow.NewDisconnectedContext(ctx)
	workflow.GetSignalChannel(disconnected, "release").Receive(disconnected, nil)
	if ctx.Err() == nil {
		return "", harness.AppErrorf("expected cancellation before release")
	}
	return "", ctx.Err()
}

var Service = func() *nexus.Service {
	s := nexus.NewService("test-service")
	s.MustRegister(&gatedCancellation{Operation: temporalnexus.NewWorkflowRunOperation(
		"block", HandlerWorkflow,
		func(ctx context.Context, id string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{ID: id}, nil
		})})
	s.MustRegister(nexus.NewSyncOperation(
		"release",
		func(ctx context.Context, id string, options nexus.StartOperationOptions) (string, error) {
			c := temporalnexus.GetClient(ctx)
			if err := c.SignalWorkflow(ctx, id, "", "release", nil); err != nil {
				return "", err
			}
			err := c.GetWorkflow(ctx, id, "").Get(ctx, nil)
			if !temporal.IsCanceledError(err) {
				return "", fmt.Errorf("expected backing workflow cancellation, got %v", err)
			}
			return "canceled", nil
		}))
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, "test-service")
	id := workflow.GetInfo(ctx).WorkflowExecution.ID + "-handler"
	opCtx, cancel := workflow.WithCancel(ctx)
	fut := nc.ExecuteOperation(opCtx, "block", id, workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
		CancellationType:       workflow.NexusOperationCancellationTypeWaitRequested,
	})
	var execution workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &execution); err != nil {
		return "", err
	}
	if execution.OperationToken == "" {
		return "", harness.AppErrorf("expected non-empty operation token")
	}
	cancel()
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	if err := workflow.ExecuteActivity(activityCtx, CancellationGate, false).Get(ctx, nil); err != nil {
		return "", err
	}
	if fut.IsReady() {
		return "", harness.AppErrorf("WAIT_REQUESTED returned before cancellation delivery")
	}
	if err := workflow.ExecuteActivity(activityCtx, CancellationGate, true).Get(ctx, nil); err != nil {
		return "", err
	}
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
	Activities:      CancellationGate,
	NexusServices:   Service,
	ExpectRunResult: "canceled",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		var releaseID int64
		scheduled, started, requested, acknowledged, completed := 0, 0, 0, 0, 0
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
					if acknowledged != 1 {
						return fmt.Errorf("WAIT_REQUESTED returned before cancellation acknowledgement")
					}
					releaseID = event.EventId
				}
			}
			switch event.EventType {
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED:
				started++
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED:
				requested++
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_COMPLETED:
				acknowledged++
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED:
				if releaseID == 0 {
					return fmt.Errorf("operation completed before the caller could release it")
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED:
				completed++
				if releaseID == 0 || event.GetNexusOperationCompletedEventAttributes().ScheduledEventId != releaseID {
					return fmt.Errorf("only the release operation may complete successfully")
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED, enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT,
				enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_FAILED:
				return fmt.Errorf("unexpected Nexus event: %v", event.EventType)
			}
		}
		if scheduled != 2 || started != 1 || requested != 1 || acknowledged != 1 || completed != 1 {
			return fmt.Errorf("unexpected lifecycle counts: scheduled=%d started=%d requested=%d acknowledged=%d completed=%d",
				scheduled, started, requested, acknowledged, completed)
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
