package cancel_try_cancel

import (
	"context"
	"time"

	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: activities,
	// Effectively disable heartbeat throttling
	// TODO(cretz): Pending https://github.com/temporalio/sdk-go/issues/859
	// WorkerOptions: worker.Options{MaxHeartbeatThrottleInterval: 1 * time.Nanosecond},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		// Put client on activities
		activities.Client = runner.Client
		return runner.ExecuteDefault(ctx)
	},
}

func Workflow(ctx workflow.Context) error {
	// Start an activity
	actCtx, actCancel := workflow.WithCancel(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 1 * time.Minute,
		HeartbeatTimeout:       5 * time.Second,
		RetryPolicy:            harness.RetryDisabled,
		// This is the default, just setting it here to be clear
		WaitForCancellation: false,
	}))
	fut := workflow.ExecuteActivity(actCtx, activities.CancellableActivity)

	// Sleep for the smallest amount of time (force task turnover)
	if err := workflow.Sleep(ctx, 1*time.Millisecond); err != nil {
		return err
	}

	// Cancel and confirm the activity errors with the cancel
	actCancel()
	if err := fut.Get(ctx, nil); !temporal.IsCanceledError(err) {
		return harness.AppErrorf("expected activity cancel error, got: %v", err)
	}

	// Confirm a signal is received that the activity was cancelled
	var result string
	workflow.GetSignalChannel(ctx, "activity-result").Receive(ctx, &result)
	if result != "cancelled" {
		return harness.AppErrorf("expected activity to get cancelled, got: %v", result)
	}
	return nil
}

var activities = &Activities{}

type Activities struct{ Client client.Client }

func (a *Activities) CancellableActivity(ctx context.Context) error {
	// Heartbeat every second for a minute
	var result string
	for i := 0; i < 60 && result == ""; i++ {
		select {
		case <-time.After(1 * time.Second):
			activity.RecordHeartbeat(ctx)
		case <-ctx.Done():
			result = "cancelled"
		}
	}
	if result == "" {
		result = "timeout"
	}
	// Send signal to workflow
	workflow := activity.GetInfo(ctx).WorkflowExecution
	return a.Client.SignalWorkflow(context.Background(), workflow.ID, workflow.RunID, "activity-result", result)
}
