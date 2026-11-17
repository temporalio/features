package async_cancellation_try_cancel

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

// A checkpoint creates a workflow task without sharing the Nexus dispatch queue.
// The second invocation opens the gate, only after the result future resolves.
func CancellationGate(ctx context.Context, release bool) error {
	if release {
		allowOnce.Do(func() { close(allowCancellation) })
	}
	return nil
}

// Delay delivery until the caller's operation future has resolved. Waiting for
// cancellation acknowledgement or operation completion would deadlock this test.
type gatedCancellation struct {
	nexus.Operation[string, string]
	allow     chan struct{}
	delivered chan struct{}
	once      sync.Once
}

func (o *gatedCancellation) Cancel(ctx context.Context, token string, options nexus.CancelOperationOptions) error {
	select {
	case <-o.allow:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := o.Operation.Cancel(ctx, token, options); err != nil {
		return err
	}
	o.once.Do(func() { close(o.delivered) })
	return nil
}

func HandlerWorkflow(ctx workflow.Context, id string) (string, error) {
	ctx.Done().Receive(ctx, nil)
	return "", ctx.Err()
}

var Service = func() *nexus.Service {
	s := nexus.NewService("test-service")
	block := &gatedCancellation{
		Operation: temporalnexus.NewWorkflowRunOperation(
			"block", HandlerWorkflow,
			func(ctx context.Context, id string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
				return client.StartWorkflowOptions{ID: id}, nil
			}),
		allow:     allowCancellation,
		delivered: make(chan struct{}),
	}
	s.MustRegister(block)
	s.MustRegister(nexus.NewSyncOperation(
		"release",
		func(ctx context.Context, id string, options nexus.StartOperationOptions) (string, error) {
			select {
			case <-block.delivered:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			err := temporalnexus.GetClient(ctx).GetWorkflow(ctx, id, "").Get(ctx, nil)
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
	// Processing the recorded cancel-request event requires another workflow
	// task. This round trip supplies one without allowing cancellation delivery.
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	if err := workflow.ExecuteActivity(activityCtx, CancellationGate, false).Get(ctx, nil); err != nil {
		return "", err
	}
	err := fut.Get(ctx, nil)
	canceled, ok := err.(*temporal.CanceledError)
	if !ok || canceled.Unwrap() != nil || canceled.HasDetails() {
		return "", harness.AppErrorf("expected leaf local cancellation, got %T: %v", err, err)
	}
	if err := workflow.ExecuteActivity(activityCtx, CancellationGate, true).Get(ctx, nil); err != nil {
		return "", err
	}
	// Wait for the backing workflow to close, through the service's own client.
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
		var openGateID int64
		scheduled, started, requested, completed, checkpoints := 0, 0, 0, 0, 0
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for hist.HasNext() {
			event, err := hist.Next()
			if err != nil {
				return err
			}
			if event.GetActivityTaskScheduledEventAttributes() != nil {
				checkpoints++
				if checkpoints == 2 {
					openGateID = event.EventId
				}
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
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED:
				requested++
				if releaseID != 0 {
					return fmt.Errorf("cancellation must be requested before release")
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_COMPLETED, enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED:
				if openGateID == 0 {
					return fmt.Errorf("TRY_CANCEL waited for cancellation delivery or completion")
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED:
				completed++
				id := event.GetNexusOperationCompletedEventAttributes().ScheduledEventId
				if releaseID == 0 || id != releaseID {
					return fmt.Errorf("only the release operation may complete successfully")
				}
			case enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED, enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT,
				enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_FAILED:
				return fmt.Errorf("unexpected Nexus event: %v", event.EventType)
			}
		}
		if scheduled != 2 || started != 1 || requested != 1 || completed != 1 || checkpoints != 2 {
			return fmt.Errorf("unexpected lifecycle counts: scheduled=%d started=%d requested=%d completed=%d checkpoints=%d", scheduled, started, requested, completed, checkpoints)
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
