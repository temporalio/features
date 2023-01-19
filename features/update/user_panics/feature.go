package user_panics

import (
	"context"

	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	execPanic     = "execPanic"
	validatePanic = "validatePanic"
	done          = "done"
)

var Feature = harness.Feature{
	Workflows: Workflow,
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
			run.GetID(),
			run.GetRunID(),
			execPanic,
		)
		runner.Require.NoError(err)
		err = handle.Get(ctx, nil)
		runner.Require.Error(err)
		runner.Require.ErrorContains(err, "update oops")

		handle, err = runner.Client.UpdateWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			validatePanic,
		)
		runner.Require.NoError(err)
		err = handle.Get(ctx, nil)
		runner.Require.Error(err)
		runner.Require.ErrorContains(err, "validator oops")

		runner.Require.NoError(
			runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), done, nil),
		)
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandlerWithOptions(ctx, execPanic,
		func(ctx workflow.Context) (int, error) {
			panic("update oops")
		},
		workflow.UpdateHandlerOptions{},
	); err != nil {
		return err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, validatePanic,
		func(ctx workflow.Context) error { return nil },
		workflow.UpdateHandlerOptions{Validator: func() error { panic("validator oops") }},
	); err != nil {
		return err
	}

	_ = workflow.GetSignalChannel(ctx, done).Receive(ctx, nil)
	return ctx.Err()
}
