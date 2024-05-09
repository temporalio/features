package task_failure

import (
	"context"
	"strings"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"github.com/temporalio/features/harness/go/history"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	panicMsg                = "fear not, dear examiner of logs, for this panic was intentional"
	panicFromExecUpdate     = "panic_from_exec"
	panicFromValidateUpdate = "panic_from_validate"
	shutdownSignal          = "shutdown!"
)

var Feature = harness.Feature{
	Workflows:                          PanickyUpdates,
	WorkerOptions:                      worker.Options{WorkflowPanicPolicy: worker.BlockWorkflow},
	DisableWorkflowPanicPolicyOverride: true,
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
			panicFromValidateUpdate,
		)
		runner.Require.NoError(err)
		err = handle.Get(ctx, nil)
		var panicErr *temporal.PanicError
		runner.Require.ErrorAs(err, &panicErr)
		runner.Require.ErrorContains(err, panicMsg)

		_, err = runner.Client.UpdateWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			panicFromExecUpdate,
		)
		runner.Require.NoError(err)

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		runner.Require.NoError(run.Get(ctx, nil))

		runner.Require.Equal(1, countPanicWFTFailures(ctx, runner),
			"update handler panic should have caused 1 WFT Failure")

		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		return run, ctx.Err()
	},
}

func countPanicWFTFailures(ctx context.Context, runner *harness.Runner) int {
	fetcher := &history.Fetcher{
		Client:         runner.Client,
		Namespace:      runner.Namespace,
		TaskQueue:      runner.TaskQueue,
		FeatureStarted: runner.CreateTime,
	}
	histories, err := fetcher.Fetch(ctx)
	runner.Require.NoError(err)
	runner.Require.NotEmpty(histories)
	count := 0
	for _, ev := range histories[0].GetEvents() {
		if attrs := ev.GetWorkflowTaskFailedEventAttributes(); attrs != nil {
			if strings.Contains(attrs.GetFailure().GetMessage(), panicMsg) &&
				attrs.GetCause() == enumspb.WORKFLOW_TASK_FAILED_CAUSE_WORKFLOW_WORKER_UNHANDLED_FAILURE {
				count++
			}
		}
	}
	return count
}

var panicUpdate = true

func PanickyUpdates(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandler(ctx, panicFromExecUpdate, func(ctx workflow.Context) error {
		// DON'T DO THIS. This panicUpdate global is not part of the workflow
		// state so reading and setting it is non-determinism. We allow
		// ourselves this transgression in this controlled test setting to
		// effect an update that panics once and then not on subsequent
		// invocations.
		if panicUpdate {
			panicUpdate = false
			panic(panicMsg)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, panicFromValidateUpdate, func(ctx workflow.Context) error {
		return nil
	}, workflow.UpdateHandlerOptions{
		Validator: func() error { panic(panicMsg) },
	}); err != nil {
		return err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return ctx.Err()
}
