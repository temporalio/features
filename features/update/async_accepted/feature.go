package async_accepted

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type UpdateDisposition int

const (
	theUpdate       = "theUpdate"
	theUpdateResult = 123
	shutdownSignal  = "shutdown_signal"

	succeed       UpdateDisposition = 0
	failWithError UpdateDisposition = 1

	requestedSleep = 2 * time.Second
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting async_accepted update test execution")

		if reason := updateutil.CheckServerSupportsAsyncAcceptedUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		// Issue an async update that should succeed after `requestedSleep`
		runner.Log.Info("Sending async update (should succeed)", "updateID", "update:1", "sleep", requestedSleep, "waitForStage", "Accepted")
		start := time.Now()
		originalHandle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				UpdateID:     "update:1",
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   theUpdate,
				Args:         []interface{}{requestedSleep, succeed},
				WaitForStage: client.WorkflowUpdateStageAccepted,
			})
		dur := time.Since(start)
		runner.Require.NoError(err)
		runner.Log.Info("Async update accepted", "updateID", "update:1", "acceptedInDuration", dur)
		runner.Require.Lessf(dur, requestedSleep, "requesting the async "+
			"update should block for less than the requested update "+
			"execution time", requestedSleep)

		// Create a separate handle to the same update
		runner.Log.Info("Creating separate handle to same update", "updateID", originalHandle.UpdateID())
		anotherHandle := runner.Client.GetWorkflowUpdateHandle(
			client.GetWorkflowUpdateHandleOptions{
				WorkflowID: run.GetID(),
				RunID:      run.GetRunID(),
				UpdateID:   originalHandle.UpdateID(),
			},
		)

		var result int
		// should block on in-flight update
		runner.Log.Info("Getting update result via separate handle (should block until completed)")
		runner.Require.NoError(anotherHandle.Get(ctx, &result))
		runner.Log.Info("Separate handle returned result", "result", result)
		runner.Require.Equal(theUpdateResult, result)

		// update has completed on server so this will look into mutable state
		// to load the outcome
		runner.Log.Info("Getting update result via original handle (should return immediately from cache)")
		runner.Require.NoError(originalHandle.Get(ctx, &result))
		runner.Log.Info("Original handle returned result", "result", result)
		runner.Require.Equal(theUpdateResult, result)

		// issue an async update that should return an error
		runner.Log.Info("Sending async update (should fail with error)", "updateID", "update:3", "waitForStage", "Accepted")
		errUpdate, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				UpdateID:     "update:3",
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   theUpdate,
				Args:         []interface{}{requestedSleep, failWithError},
				WaitForStage: client.WorkflowUpdateStageAccepted,
			})
		runner.Require.NoError(err)
		runner.Log.Info("Error update accepted, waiting for completion")
		err = errUpdate.Get(ctx, nil)
		var errErr *temporal.ApplicationError
		runner.Log.Info("Error update completed with expected error", "errorType", fmt.Sprintf("%T", err))
		runner.Require.ErrorAs(err, &errErr, "error type was %T", err)

		// issue an update that will succeed after `requestedSleep`
		runner.Log.Info("Sending async update (should succeed but testing timeout)", "updateID", "update:4", "sleep", requestedSleep, "waitForStage", "Accepted")
		lastUpdate, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				UpdateID:     "update:4",
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   theUpdate,
				Args:         []interface{}{requestedSleep, succeed},
				WaitForStage: client.WorkflowUpdateStageAccepted,
			})
		runner.Require.NoError(err)
		runner.Log.Info("Final update accepted, testing timeout behavior")
		timeoutctx, _ := context.WithTimeout(ctx, time.Duration(float64(requestedSleep)*0.1))
		// `requestedSleep` is longer than the ctx timeout so we expect this
		// handle.Get to fail timeout before returning an outcome.
		err = lastUpdate.Get(timeoutctx, nil)
		var timeoutError *serviceerror.DeadlineExceeded
		runner.Log.Info("Final update timed out as expected", "errorType", fmt.Sprintf("%T", err))
		runner.Require.ErrorAsf(err, &timeoutError, "error type was %T", err)

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		runner.Log.Info("Async_accepted update test completed successfully")
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Workflow started, setting up update handler")

	if err := workflow.SetUpdateHandler(ctx, theUpdate,
		func(ctx workflow.Context, dur time.Duration, disp UpdateDisposition) (int, error) {
			logger.Info("Update handler invoked", "duration", dur, "disposition", disp)
			switch disp {
			case succeed:
				logger.Info("Sleeping before returning success", "duration", dur)
				workflow.Sleep(ctx, dur)
				logger.Info("Sleep completed, returning success")
			case failWithError:
				logger.Info("Returning error as requested")
				return 0, errors.New("I was told I should fail")
			}
			return theUpdateResult, nil
		},
	); err != nil {
		return err
	}
	logger.Info("Update handler registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing")
	return ctx.Err()
}
