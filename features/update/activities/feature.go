package activities

import (
	"context"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	updateActivity = "updateActivity"
	activityResult = 6
	activityCount  = 5
	done           = "done"
	shutdownSignal = "shutdown_signal"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: SomeActivity,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   updateActivity,
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)

		var result int
		runner.Require.NoError(handle.Get(ctx, &result))
		runner.Require.Equal(activityResult*activityCount, result)

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		return run, ctx.Err()
	},
}

func SomeActivity(ctx context.Context) (int, error) {
	return activityResult, nil
}

func Workflow(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateActivity,
		func(ctx workflow.Context) (int, error) {
			selector := workflow.NewSelector(ctx)
			aopts := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Second}
			total := 0
			for i := 0; i < activityCount; i++ {
				selector.AddFuture(
					workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, aopts), SomeActivity),
					func(f workflow.Future) {
						var result int
						_ = f.Get(ctx, &result)
						total += result
					},
				)
			}

			for i := 0; i < activityCount; i++ {
				selector.Select(ctx)
			}

			return total, nil
		},
		workflow.UpdateHandlerOptions{},
	); err != nil {
		return err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return ctx.Err()
}
