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
				UpdateName:   updateAdd,
				Args:         []interface{}{-1},
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		var result int
		runner.Require.Error(handle.Get(ctx, &result), "expected negative value to be rejected")

		for i := 0; i < count; i++ {
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
		}

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
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
	counter := 0
	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateAdd,
		func(ctx workflow.Context, i int) (int, error) {
			counter += i
			return counter, nil
		},
		workflow.UpdateHandlerOptions{Validator: nonNegative},
	); err != nil {
		return 0, err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return counter, nil
}
