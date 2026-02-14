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
		runner.Log.Info("Starting activities update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending update to workflow (will spawn activities)", "updateName", updateActivity, "waitForStage", "Completed")
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
		runner.Log.Info("Update request returned", "updateID", handle.UpdateID())

		runner.Log.Info("Getting update result")
		var result int
		runner.Require.NoError(handle.Get(ctx, &result))
		runner.Log.Info("Update result received", "result", result, "expectedResult", activityResult*activityCount)
		runner.Require.Equal(activityResult*activityCount, result)

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		runner.Log.Info("Activities update test completed successfully")
		return run, ctx.Err()
	},
}

func SomeActivity(ctx context.Context) (int, error) {
	return activityResult, nil
}

func Workflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Workflow started, setting up update handler")

	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateActivity,
		func(ctx workflow.Context) (int, error) {
			logger.Info("Update handler invoked, spawning activities", "activityCount", activityCount)
			selector := workflow.NewSelector(ctx)
			aopts := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Second}
			total := 0
			for i := 0; i < activityCount; i++ {
				logger.Info("Scheduling activity", "activityIndex", i)
				selector.AddFuture(
					workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, aopts), SomeActivity),
					func(f workflow.Future) {
						var result int
						_ = f.Get(ctx, &result)
						logger.Info("Activity completed", "result", result)
						total += result
					},
				)
			}

			logger.Info("Waiting for all activities to complete")
			for i := 0; i < activityCount; i++ {
				selector.Select(ctx)
			}
			logger.Info("All activities completed", "total", total)

			return total, nil
		},
		workflow.UpdateHandlerOptions{},
	); err != nil {
		return err
	}
	logger.Info("Update handler registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing")
	return ctx.Err()
}
