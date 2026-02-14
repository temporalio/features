package deduplication

import (
	"context"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	reusedUpdateID = "reused_update_id"
	incrementCount = "incrementCount"
	shutdownSignal = "shutdown_signal"
	expectedCount  = 1
)

var Feature = harness.Feature{
	Workflows: Count,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting deduplication update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending first update with reusable ID", "updateID", reusedUpdateID, "updateName", incrementCount, "waitForStage", "Accepted")
		handle1, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				UpdateID:     reusedUpdateID,
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   incrementCount,
				WaitForStage: client.WorkflowUpdateStageAccepted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("First update accepted")

		runner.Log.Info("Sending second update with same ID (should deduplicate)", "updateID", reusedUpdateID, "updateName", incrementCount, "waitForStage", "Accepted")
		handle2, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				UpdateID:     reusedUpdateID,
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   incrementCount,
				WaitForStage: client.WorkflowUpdateStageAccepted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("Second update accepted (deduplicated)")

		runner.Log.Info("Getting result from first handle")
		var result int
		runner.Require.NoError(handle1.Get(ctx, &result))
		runner.Log.Info("First handle returned result", "result", result, "expectedCount", expectedCount)
		runner.Require.Equal(expectedCount, result)

		runner.Log.Info("Getting result from second handle (should be same)")
		runner.Require.NoError(handle2.Get(ctx, &result))
		runner.Log.Info("Second handle returned result", "result", result, "expectedCount", expectedCount)
		runner.Require.Equal(expectedCount, result)

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)

		runner.Log.Info("Verifying only one update was actually executed")
		nUpdates, err := harness.GetCountCompletedUpdates(ctx, runner.Client, run.GetID())
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Completed updates count verified", "actualCount", nUpdates, "expectedCount", expectedCount)
		runner.Require.Equal(expectedCount, nUpdates)

		runner.Log.Info("Deduplication update test completed successfully")
		return run, ctx.Err()
	},
}

func Count(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Count workflow started, setting up update handler")

	counter := 0

	err := workflow.SetUpdateHandler(
		ctx,
		incrementCount,
		func(ctx workflow.Context) (int, error) {
			logger.Info("Update handler invoked, incrementing counter", "currentCounter", counter)
			counter += 1
			logger.Info("Counter incremented, sleeping", "newCounter", counter)
			// Check that deduplication does not need completion
			err := workflow.Sleep(ctx, time.Second)
			if err != nil {
				logger.Error("Sleep failed", "error", err)
				return counter, err
			}
			logger.Info("Update handler completed", "finalCounter", counter)

			return counter, nil
		},
	)
	if err != nil {
		return err
	}
	logger.Info("Update handler registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing")
	return ctx.Err()
}
