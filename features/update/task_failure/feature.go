package task_failure

import (
	"context"
	"os"
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
		runner.Log.Info("Starting task_failure update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution (will test panic handling)")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending update that will panic in validator", "updateName", panicFromValidateUpdate, "waitForStage", "Completed")
		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   panicFromValidateUpdate,
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("Getting result (should receive panic error)")
		err = handle.Get(ctx, nil)
		var panicErr *temporal.PanicError
		runner.Require.ErrorAs(err, &panicErr)
		runner.Require.ErrorContains(err, panicMsg)
		runner.Log.Info("Received expected panic error from validator")

		runner.Log.Info("Sending update that will panic in executor", "updateName", panicFromExecUpdate, "waitForStage", "Completed")
		_, err = runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   panicFromExecUpdate,
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("Update sent (will panic once then retry)")

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		runner.Log.Info("Waiting for workflow completion")
		runner.Require.NoError(run.Get(ctx, nil))
		runner.Log.Info("Workflow completed")

		if os.Getenv("TEMPORAL_FEATURES_DISABLE_WORKFLOW_COMPLETION_CHECK") != "" {
			runner.Log.Info("Verifying panic caused workflow task failure")
			panicCount := countPanicWFTFailures(ctx, runner)
			runner.Log.Info("Panic WFT failures counted", "count", panicCount, "expected", 1)
			runner.Require.Equal(1, panicCount,
				"update handler panic should have caused 1 WFT Failure")
		}

		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		runner.Log.Info("Task_failure update test completed successfully")
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
	logger := workflow.GetLogger(ctx)
	logger.Info("PanickyUpdates workflow started, setting up panic handlers")

	if err := workflow.SetUpdateHandler(ctx, panicFromExecUpdate, func(ctx workflow.Context) error {
		logger.Info("Panic executor handler invoked")
		// DON'T DO THIS. This panicUpdate global is not part of the workflow
		// state so reading and setting it is non-determinism. We allow
		// ourselves this transgression in this controlled test setting to
		// effect an update that panics once and then not on subsequent
		// invocations.
		if panicUpdate {
			logger.Info("Triggering panic (first invocation)")
			panicUpdate = false
			panic(panicMsg)
		}
		logger.Info("Panic executor handler completed (second invocation, no panic)")
		return nil
	}); err != nil {
		return err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, panicFromValidateUpdate, func(ctx workflow.Context) error {
		logger.Info("Panic validator handler invoked (should not reach here)")
		return nil
	}, workflow.UpdateHandlerOptions{
		Validator: func() error {
			logger.Info("Validator invoked, triggering panic")
			panic(panicMsg)
		},
	}); err != nil {
		return err
	}
	logger.Info("Update handlers registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing")
	return ctx.Err()
}
