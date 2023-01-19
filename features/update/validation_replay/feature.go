package validation_replay

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	update = "doSomeStuff"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: TheActivity,
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
			update,
		)
		runner.Require.NoError(err)
		runner.Require.NoError(handle.Get(ctx, nil))

		updateutil.RequestShutdown(ctx, runner, run)
		return run, nil
	},
}

var validationCounter = 0

// Workflow hosts an update handler that is intentionally broken such that
// validation passes on the first invocation but not subsequent invokcations.
// The SDK under test must skip update validation during replay. If it does not,
// the second call to the Validator here (which will happen during replay) will
// fail and therefore the update will not run, causing the commands generated by
// the replay to diverge from the original events and thus replay to fail.
func Workflow(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandlerWithOptions(ctx, update,
		func(ctx workflow.Context) error {
			var result int
			aopts := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Second}
			workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, aopts), TheActivity).Get(ctx, &result)
			return nil
		},
		workflow.UpdateHandlerOptions{Validator: func() error {
			// Don't do this! We only touch this global from within a validation
			// handler for test purposes so that this validation function fails
			// if it is called a second time (as it would be if it were to be
			// called during replay)
			validationCounter++
			if validationCounter > 1 {
				return errors.New("failing validation")
			}
			return nil
		}},
	); err != nil {
		return err
	}
	updateutil.AwaitShutdown(ctx)
	return nil
}

func TheActivity(ctx context.Context) (int, error) {
	return 1, nil
}
