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
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

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

		var result int
		runner.Require.NoError(handle1.Get(ctx, &result))
		runner.Require.Equal(expectedCount, result)

		runner.Require.NoError(handle2.Get(ctx, &result))
		runner.Require.Equal(expectedCount, result)

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)

		nUpdates, err := harness.GetCountCompletedUpdates(ctx, runner.Client, run.GetID())
		if err != nil {
			return nil, err
		}
		runner.Require.Equal(expectedCount, nUpdates)

		return run, ctx.Err()
	},
}

func Count(ctx workflow.Context) error {
	counter := 0

	err := workflow.SetUpdateHandler(
		ctx,
		incrementCount,
		func(ctx workflow.Context) (int, error) {
			counter += 1
			// Check that deduplication does not need completion
			err := workflow.Sleep(ctx, time.Second)
			if err != nil {
				return counter, err
			}

			return counter, nil
		},
	)
	if err != nil {
		return err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return ctx.Err()
}
