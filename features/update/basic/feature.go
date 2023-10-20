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
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

		updateHandle, err := runner.Client.UpdateWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			myUpdateName,
			myUpdateArg,
		)
		runner.Require.NoError(err)

		var updateResult string
		runner.Require.NoError(updateHandle.Get(ctx, &updateResult))
		runner.Require.Equal(fmt.Sprintf("update-result:%s", myUpdateArg), updateResult)

		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	workflowResult := ""
	if err := workflow.SetUpdateHandler(ctx, myUpdateName,
		func(arg string) (string, error) {
			workflowResult = arg
			return fmt.Sprintf("update-result:%s", arg), nil
		},
	); err != nil {
		return "", err
	}
	workflow.Await(ctx, func() bool {
		return workflowResult != ""
	})

	return fmt.Sprintf("workflow-result:%s", workflowResult), nil
}
