package async_accepted

import (
	"context"
	"errors"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	updatepb "go.temporal.io/api/update/v1"
	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
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
		if reason := updateutil.CheckServerSupportsAsyncAcceptedUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

		// Issue an asyc update that should succeed after `requestedSleep`
		start := time.Now()
		originalHandle, err := runner.Client.UpdateWorkflowWithOptions(
			ctx,
			&client.UpdateWorkflowWithOptionsRequest{
				UpdateID:   "update:1",
				WorkflowID: run.GetID(),
				RunID:      run.GetRunID(),
				UpdateName: theUpdate,
				Args:       []interface{}{requestedSleep, succeed},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enumspb.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
		dur := time.Since(start)
		runner.Require.NoError(err)
		runner.Require.Lessf(dur, requestedSleep, "requesting the async "+
			"update should block for less than the requested update "+
			"execution time", requestedSleep)

		// Create a separate handle to the same update
		anotherHandle := runner.Client.GetWorkflowUpdateHandle(
			client.GetWorkflowUpdateHandleOptions{
				WorkflowID: run.GetID(),
				RunID:      run.GetRunID(),
				UpdateID:   originalHandle.UpdateID(),
			},
		)

		var result int
		// should block on in-flight update
		runner.Require.NoError(anotherHandle.Get(ctx, &result))
		runner.Require.Equal(theUpdateResult, result)

		// update has completed on server so this will look into mutable state
		// to load the outcome
		runner.Require.NoError(originalHandle.Get(ctx, &result))
		runner.Require.Equal(theUpdateResult, result)

		// issue an async update that should return an error
		errUpdate, err := runner.Client.UpdateWorkflowWithOptions(
			ctx,
			&client.UpdateWorkflowWithOptionsRequest{
				UpdateID:   "update:3",
				WorkflowID: run.GetID(),
				RunID:      run.GetRunID(),
				UpdateName: theUpdate,
				Args:       []interface{}{requestedSleep, failWithError},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enumspb.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
		runner.Require.NoError(err)
		err = errUpdate.Get(ctx, nil)
		var errErr *temporal.ApplicationError
		runner.Require.ErrorAs(err, &errErr, "error type was %T", err)

		// issue an update that will succeed after `requestedSleep`
		lastUpdate, err := runner.Client.UpdateWorkflowWithOptions(
			ctx,
			&client.UpdateWorkflowWithOptionsRequest{
				UpdateID:   "update:4",
				WorkflowID: run.GetID(),
				RunID:      run.GetRunID(),
				UpdateName: theUpdate,
				Args:       []interface{}{requestedSleep, succeed},
				WaitPolicy: &updatepb.WaitPolicy{
					LifecycleStage: enumspb.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
				},
			})
		runner.Require.NoError(err)
		timeoutctx, _ := context.WithTimeout(ctx, time.Duration(float64(requestedSleep)*0.1))
		// `requestedSleep` is longer than the ctx timeout so we expect this
		// handle.Get to fail timeout before returning an outcome.
		err = lastUpdate.Get(timeoutctx, nil)
		var timeoutError *serviceerror.DeadlineExceeded
		runner.Require.ErrorAsf(err, &timeoutError, "error type was %T", err)

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) error {
	if err := workflow.SetUpdateHandler(ctx, theUpdate,
		func(ctx workflow.Context, dur time.Duration, disp UpdateDisposition) (int, error) {
			switch disp {
			case succeed:
				workflow.Sleep(ctx, dur)
			case failWithError:
				return 0, errors.New("I was told I should fail")
			}
			return theUpdateResult, nil
		},
	); err != nil {
		return err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return ctx.Err()
}
