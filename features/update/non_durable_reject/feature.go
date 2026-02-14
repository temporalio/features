package non_durable_reject

import (
	"context"
	"fmt"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	step           = 2
	count          = 5
	updateAdd      = "updateActivity"
	shutdownSignal = "shutdown_signal"
)

var Feature = harness.Feature{
	Workflows:       NonDurableReject,
	ExpectRunResult: step * count,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting non_durable_reject update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending update with negative value (should be rejected)", "updateName", updateAdd, "arg", -1, "waitForStage", "Completed")
		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   updateAdd,
				Args:         []interface{}{-1},
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		var result int
		runner.Log.Info("Getting result (should fail)")
		runner.Require.Error(handle.Get(ctx, &result), "expected negative value to be rejected")
		runner.Log.Info("Update correctly rejected")

		for i := 0; i < count; i++ {
			runner.Log.Info("Iteration starting", "iteration", i, "totalIterations", count)

			runner.Log.Info("Sending update with positive value (should be accepted)", "updateName", updateAdd, "arg", step, "iteration", i)
			handle, err := runner.Client.UpdateWorkflow(
				ctx,
				client.UpdateWorkflowOptions{
					WorkflowID:   run.GetID(),
					RunID:        run.GetRunID(),
					UpdateName:   updateAdd,
					Args:         []interface{}{step},
					WaitForStage: client.WorkflowUpdateStageCompleted,
				},
			)
			runner.Require.NoError(err)
			runner.Require.NoError(handle.Get(ctx, &result), "expected non-negative value to be accepted")
			runner.Log.Info("Positive update accepted", "result", result, "iteration", i)

			runner.Log.Info("Sending update with negative value (should be rejected)", "updateName", updateAdd, "arg", -1, "iteration", i)
			handle, err = runner.Client.UpdateWorkflow(
				ctx,
				client.UpdateWorkflowOptions{
					WorkflowID:   run.GetID(),
					RunID:        run.GetRunID(),
					UpdateName:   updateAdd,
					Args:         []interface{}{-1},
					WaitForStage: client.WorkflowUpdateStageCompleted,
				},
			)
			runner.Require.NoError(err)
			runner.Require.Error(handle.Get(ctx, &result), "expected negative value to be rejected")
			runner.Log.Info("Negative update correctly rejected", "iteration", i)
		}

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		runner.Log.Info("Non_durable_reject update test completed successfully")
		return run, nil
	},
}

func nonNegative(i int) error {
	if i < 0 {
		return fmt.Errorf("expected non-negative value (%v)", i)
	}
	return nil
}

func NonDurableReject(ctx workflow.Context) (int, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("NonDurableReject workflow started, setting up update handler with validator")

	counter := 0
	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateAdd,
		func(ctx workflow.Context, i int) (int, error) {
			logger.Info("Update handler invoked", "arg", i, "currentCounter", counter)
			counter += i
			logger.Info("Update handler completed", "newCounter", counter)
			return counter, nil
		},
		workflow.UpdateHandlerOptions{Validator: nonNegative},
	); err != nil {
		return 0, err
	}
	logger.Info("Update handler with validator registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing", "finalCounter", counter)
	return counter, nil
}
