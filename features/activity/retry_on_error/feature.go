package retry_on_error

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:           Workflow,
	Activities:          AlwaysFailActivity,
	ExpectActivityError: "activity attempt 5 failed",
}

func Workflow(ctx workflow.Context) error {
	// Allow 4 retries with no backoff
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			// Retry immediately
			InitialInterval: 1 * time.Nanosecond,
			// Do not increase retry backoff each time
			BackoffCoefficient: 1,
			// 5 total maximum attempts
			MaximumAttempts: 5,
		},
	})

	// Execute activity and return error
	return workflow.ExecuteActivity(ctx, AlwaysFailActivity).Get(ctx, nil)
}

func AlwaysFailActivity(ctx context.Context) error {
	return fmt.Errorf("activity attempt %v failed", activity.GetInfo(ctx).Attempt)
}
