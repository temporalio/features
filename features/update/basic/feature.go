package activities

import (
	"context"
	"fmt"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	myUpdateName = "my-update-name"
	myUpdateArg  = "my-update-arg"
)

var Feature = harness.Feature{
	Workflows:       Workflow,
	ExpectRunResult: fmt.Sprintf("workflow-result:%s", myUpdateArg),
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting basic update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending update to workflow", "updateName", myUpdateName, "arg", myUpdateArg, "waitForStage", "Completed")
		updateHandle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   myUpdateName,
				Args:         []interface{}{myUpdateArg},
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("Update request returned", "updateID", updateHandle.UpdateID())

		runner.Log.Info("Getting update result")
		var updateResult string
		runner.Require.NoError(updateHandle.Get(ctx, &updateResult))
		runner.Log.Info("Update result received", "result", updateResult)
		runner.Require.Equal(fmt.Sprintf("update-result:%s", myUpdateArg), updateResult)

		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		runner.Log.Info("Basic update test completed successfully")
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Workflow started, setting up update handler")

	workflowResult := ""
	if err := workflow.SetUpdateHandler(ctx, myUpdateName,
		func(ctx workflow.Context, arg string) (string, error) {
			logger.Info("Update handler invoked", "arg", arg)
			workflowResult = arg
			result := fmt.Sprintf("update-result:%s", arg)
			logger.Info("Update handler completed", "result", result)
			return result, nil
		},
	); err != nil {
		return "", err
	}
	logger.Info("Update handler registered, waiting for update")

	workflow.Await(ctx, func() bool {
		return workflowResult != ""
	})
	logger.Info("Update received, workflow completing", "workflowResult", workflowResult)

	return fmt.Sprintf("workflow-result:%s", workflowResult), nil
}
