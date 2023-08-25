package backfill

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/enums/v1"
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

	// Run backfill
	now := time.Now()
	threeYearsAgo := now.Add(-3 * 365 * 24 * time.Hour).Truncate(time.Minute)
	thirtyMinutesAgo := now.Add(-30 * time.Minute).Truncate(time.Minute)
	err = handle.Backfill(ctx, client.ScheduleBackfillOptions{
		Backfill: []client.ScheduleBackfill{
			{
				Start: threeYearsAgo.Add(-2 * time.Minute), End: threeYearsAgo,
				Overlap: enums.SCHEDULE_OVERLAP_POLICY_ALLOW_ALL,
			},
			{
				Start: thirtyMinutesAgo.Add(-2 * time.Minute), End: thirtyMinutesAgo,
				Overlap: enums.SCHEDULE_OVERLAP_POLICY_ALLOW_ALL,
			},
		},
	})
	r.Require.NoError(err)

	// Confirm 4 executions
	r.Require.Eventually(func() bool {
		desc, err := handle.Describe(ctx)
		r.Require.NoError(err)
		return desc.Info.NumActions == 4 && len(desc.Info.RunningWorkflows) == 0
	}, 5*time.Second, 1*time.Second)
	return nil, nil
}
