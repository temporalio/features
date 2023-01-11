package trigger

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute:   Execute,
}

func Workflow(ctx workflow.Context, arg string) (string, error) { return arg, nil }

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Create paused 1m schedule
	workflowID := uuid.NewString()
	handle, err := r.Client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID:   uuid.NewString(),
		Spec: client.ScheduleSpec{Intervals: []client.ScheduleIntervalSpec{{Every: 1 * time.Minute}}},
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			Workflow:  Workflow,
			Args:      []interface{}{"arg1"},
			TaskQueue: r.TaskQueue,
		},
		Paused: true,
	})
	r.Require.NoError(err)

	// Remove the schedule on complete
	defer func() {
		if err := handle.Delete(context.Background()); err != nil {
			r.Log.Warn("Failed deleting schedule handle", "error", err)
		}
	}()

	// Trigger it
	r.Require.NoError(handle.Trigger(ctx, client.ScheduleTriggerOptions{}))
	// TODO(cretz): We have to wait before triggering again. See
	// https://github.com/temporalio/temporal/issues/3614
	time.Sleep(2 * time.Second)
	r.Require.NoError(handle.Trigger(ctx, client.ScheduleTriggerOptions{}))

	// Confirm 2 ran
	r.Require.Eventually(func() bool {
		desc, err := handle.Describe(ctx)
		r.Require.NoError(err)
		return desc.Info.NumActions == 2
	}, 3*time.Second, 100*time.Millisecond)
	return nil, nil
}
