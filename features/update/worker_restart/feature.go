package worker_restart

import (
	"context"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	fetchAndAdd               = "fetchAndAdd"
	done                      = "done"
	addend                    = 1
	updateNotEnabledErrorType = "PermissionDenied"
	shutdownSignal            = "shutdown_signal"
)

var Feature = harness.Feature{
	Workflows:       Workflow,
	Activities:      Block,
	ExpectRunResult: 0 + addend,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting worker_restart update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		if temporal.SDKVersion == "1.21.0" || temporal.SDKVersion == "1.21.1" {
			return nil, runner.Skip("known to be broken in sdk-go v" + temporal.SDKVersion)
		}

		runner.Log.Info("Starting workflow execution")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		updateErr := make(chan error, 1)
		updateResult := make(chan int, 1)
		runner.Log.Info("Starting goroutine to send update (will block in activity)")
		go func() {
			runner.Log.Info("Goroutine: Sending update", "updateName", fetchAndAdd, "arg", addend, "waitForStage", "Completed")
			handle, err := runner.Client.UpdateWorkflow(
				ctx,
				client.UpdateWorkflowOptions{
					WorkflowID:   run.GetID(),
					RunID:        run.GetRunID(),
					UpdateName:   fetchAndAdd,
					Args:         []interface{}{addend},
					WaitForStage: client.WorkflowUpdateStageCompleted,
				},
			)
			var result int
			if err != nil {
				runner.Log.Error("Goroutine: Update request failed", "error", err)
				updateErr <- err
			} else if err := handle.Get(ctx, &result); err != nil {
				runner.Log.Error("Goroutine: Update result failed", "error", err)
				updateErr <- err
			} else {
				runner.Log.Info("Goroutine: Update completed", "result", result)
				updateResult <- result
			}
		}()

		runner.Log.Info("Waiting for update to start (activity to begin)")
		<-updateStarted
		runner.Log.Info("Update started, stopping worker")
		runner.StopWorker()
		runner.Log.Info("Worker stopped, sleeping for 1 second")
		time.Sleep(time.Second)
		runner.Log.Info("Allowing activity to complete")
		close(updateContinue)
		runner.Log.Info("Starting worker again")
		runner.Require.NoError(runner.StartWorker())
		runner.Log.Info("Worker restarted")

		runner.Log.Info("Waiting for update result or error")
		select {
		case result := <-updateResult:
			runner.Log.Info("Update result received", "result", result, "expected", 0)
			runner.Require.Equal(result, 0)
		case err := <-updateErr:
			runner.Log.Error("Update error received", "error", err)
			return run, err
		}
		runner.Log.Info("Sleeping for 1 second before shutdown")
		time.Sleep(time.Second)
		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		runner.Log.Info("Worker_restart update test completed successfully")
		return run, ctx.Err()
	},
}

var updateStarted = make(chan struct{})
var updateContinue = make(chan struct{})

func Block(ctx context.Context) error {
	close(updateStarted)
	<-updateContinue
	return nil
}

func Workflow(ctx workflow.Context) (int, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Worker restart workflow started, setting up update handler")

	counter := 0
	if err := workflow.SetUpdateHandler(ctx, fetchAndAdd,
		func(ctx workflow.Context, i int) (int, error) {
			logger.Info("Update handler invoked", "arg", i, "currentCounter", counter)
			actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{ScheduleToCloseTimeout: 10 * time.Second})
			logger.Info("Executing blocking activity (worker will restart during this)")
			if err := workflow.ExecuteActivity(actx, Block).Get(ctx, nil); err != nil {
				logger.Error("Activity failed", "error", err)
				return 0, err
			}
			logger.Info("Activity completed, updating counter")
			tmp := counter
			counter += i
			logger.Info("Update handler completed", "returnValue", tmp, "newCounter", counter)
			return tmp, nil
		},
	); err != nil {
		return 0, err
	}
	logger.Info("Update handler registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing", "finalCounter", counter)
	return counter, ctx.Err()
}
