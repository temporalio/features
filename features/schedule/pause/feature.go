package pause

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk-features/harness/go/harness"
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
		Note:   "initial note",
	})
	r.Require.NoError(err)

	// Remove the schedule on complete
	defer func() {
		if err := handle.Delete(context.Background()); err != nil {
			r.Log.Warn("Failed deleting schedule handle", "error", err)
		}
	}()

	// Helper
	assertState := func(paused bool, note string) {
		desc, err := handle.Describe(ctx)
		r.Require.NoError(err)
		r.Require.Equal(paused, desc.Schedule.State.Paused)
		r.Require.Equal(note, desc.Schedule.State.Note)
	}

	// Confirm pause
	assertState(true, "initial note")
	// Re-pause
	r.Require.NoError(handle.Pause(ctx, client.SchedulePauseOptions{Note: "custom note1"}))
	assertState(true, "custom note1")
	// Unpause
	r.Require.NoError(handle.Unpause(ctx, client.ScheduleUnpauseOptions{}))
	assertState(false, "Unpaused via Go SDK")
	// Re-unpause
	r.Require.NoError(handle.Unpause(ctx, client.ScheduleUnpauseOptions{Note: "custom note2"}))
	assertState(false, "custom note2")
	// Pause
	r.Require.NoError(handle.Pause(ctx, client.SchedulePauseOptions{}))
	assertState(true, "Paused via Go SDK")
	return nil, nil
}
