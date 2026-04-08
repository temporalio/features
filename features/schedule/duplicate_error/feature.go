package duplicate_error

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute:   Execute,
}

func Workflow(ctx workflow.Context) error { return nil }

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	scheduleID := uuid.NewString()
	opts := client.ScheduleOptions{
		ID:   scheduleID,
		Spec: client.ScheduleSpec{Intervals: []client.ScheduleIntervalSpec{{Every: 1 * time.Hour}}},
		Action: &client.ScheduleWorkflowAction{
			ID:        uuid.NewString(),
			Workflow:  Workflow,
			TaskQueue: r.TaskQueue,
		},
		Paused: true,
	}

	handle, err := r.Client.ScheduleClient().Create(ctx, opts)
	r.Require.NoError(err)
	defer func() {
		if err := handle.Delete(context.Background()); err != nil {
			r.Log.Warn("Failed deleting schedule handle", "error", err)
		}
	}()

	// Creating again with the same schedule ID should return ErrScheduleAlreadyRunning.
	_, err = r.Client.ScheduleClient().Create(ctx, opts)
	r.Require.Error(err)
	r.Require.True(errors.Is(err, temporal.ErrScheduleAlreadyRunning),
		"expected ErrScheduleAlreadyRunning, got: %v", err)

	return nil, nil
}
