package basic

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: BasicScheduleWorkflow,
	Execute:   Execute,
}

func BasicScheduleWorkflow(ctx workflow.Context, arg string) (string, error) {
	return arg, nil
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Start schedule every 2s
	workflowID := uuid.NewString()
	handle, err := r.Client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID:      uuid.NewString(),
		Spec:    client.ScheduleSpec{Intervals: []client.ScheduleIntervalSpec{{Every: 2 * time.Second}}},
		Overlap: enums.SCHEDULE_OVERLAP_POLICY_BUFFER_ONE,
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			Workflow:  BasicScheduleWorkflow,
			Args:      []interface{}{"arg1"},
			TaskQueue: r.TaskQueue,
		},
	})
	r.Require.NoError(err)

	// Remove the schedule on complete
	defer func() {
		if err := handle.Delete(context.Background()); err != nil {
			r.Log.Warn("Failed deleting schedule handle", "error", err)
		}
	}()

	// Confirm simple describe
	desc, err := handle.Describe(ctx)
	r.Require.NoErrorf(err,
		"Describing the schedule should not fail. Schedule ID: %s workflow id: %s", handle.GetID(), workflowID)
	r.Require.Equal(workflowID, desc.Schedule.Action.(*client.ScheduleWorkflowAction).ID)

	// Confirm simple list
	// Advanced visibility is eventually consistent. See https://github.com/temporalio/features/issues/182
	listingErr := harness.RetryFor(10, 1*time.Second, func() (bool, error) {
		iter, err := r.Client.ScheduleClient().List(ctx, client.ScheduleListOptions{})
		// We don't want to retry an error calling list itself - only not finding the schedule
		r.Require.NoError(err)
		foundSchedule := false
		for iter.HasNext() && !foundSchedule {
			e, err := iter.Next()
			r.Require.NoError(err)
			foundSchedule = e.ID == handle.GetID()
		}
		return foundSchedule, nil
	})
	r.Require.NoError(listingErr)

	// Wait for first completion
	waitCompletedWith(ctx, r, workflowID, "arg1")

	// Update and change arg
	err = handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(in client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			update := &client.ScheduleUpdate{Schedule: &in.Description.Schedule}
			action := update.Schedule.Action.(*client.ScheduleWorkflowAction)
			action.Args = []interface{}{"arg2"}
			update.Schedule.Action = action
			return update, nil
		},
	})
	r.Require.NoError(err)

	// Wait for next completion
	waitCompletedWith(ctx, r, workflowID, "arg2")
	return nil, nil
}

func waitCompletedWith(ctx context.Context, r *harness.Runner, id string, untilResult string) {
	// Wait a max of 10s, waiting 1s between non-error tries
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		// Get all completed
		req := &workflowservice.ListWorkflowExecutionsRequest{
			// We cannot use WorkflowId because the schedule workflows append
			// timestamp to the ID
			Query: "WorkflowType = 'BasicScheduleWorkflow'",
		}
		var resp *workflowservice.ListWorkflowExecutionsResponse
		for resp == nil || len(req.NextPageToken) > 0 {
			resp, err := r.Client.ListWorkflow(ctx, req)
			r.Require.NoError(err)
			for _, exec := range resp.Executions {
				// We only care about ones that start with our workflow ID
				if !strings.HasPrefix(exec.Execution.WorkflowId, id) {
					continue
				}
				if exec.Status == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
					var result string
					r.Require.NoError(
						r.Client.GetWorkflow(ctx, exec.Execution.WorkflowId, exec.Execution.RunId).Get(ctx, &result))
					if result == untilResult {
						return
					}
				} else {
					r.Require.Equal(enums.WORKFLOW_EXECUTION_STATUS_RUNNING, exec.Status)
				}
			}
			req.NextPageToken = resp.NextPageToken
		}
		time.Sleep(1 * time.Second)
	}
}
