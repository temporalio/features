package non_remote_activities_worker

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Start a worker with activities registered and non-local activities disabled
var Feature = harness.Feature{
	WorkerOptions: worker.Options{LocalActivityWorkerOnly: true},
	Workflows:     Workflow,
	Activities:    Dummy,
}

func Workflow(ctx workflow.Context) error {
	// Run a workflow that schedules a single activity with short schedule-to-close timeout
	// Pick a long enough timeout for busy CI but not too long to get feedback quickly
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 3 * time.Second,
	})

	err := workflow.ExecuteActivity(ctx, Dummy).Get(ctx, nil)
	// Catch activity failure in the workflow, check that it is caused by schedule-to-start timeout
	if err == nil {
		return fmt.Errorf("expected activity to time out")
	}
	if !temporal.IsTimeoutError(err) {
		return fmt.Errorf("expected a TimeoutError, got: %s", err)
	}
	timeoutType := err.(*temporal.ActivityError).Unwrap().(*temporal.TimeoutError).TimeoutType()
	if timeoutType != enums.TIMEOUT_TYPE_SCHEDULE_TO_START {
		return fmt.Errorf("expected SCHEDULE_TO_START timeout, got: %s", timeoutType.String())
	}
	return nil
}

func Dummy(ctx context.Context) error {
	return nil
}
