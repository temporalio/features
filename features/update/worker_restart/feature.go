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
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		if temporal.SDKVersion == "1.21.0" || temporal.SDKVersion == "1.21.1" {
			return nil, runner.Skip("known to be broken in sdk-go v" + temporal.SDKVersion)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

		updateErr := make(chan error, 1)
		updateResult := make(chan int, 1)
		go func() {
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
				updateErr <- err
			} else if err := handle.Get(ctx, &result); err != nil {
				updateErr <- err
			} else {
				updateResult <- result
			}
		}()

		<-updateStarted
		runner.StopWorker()
		time.Sleep(time.Second)
		close(updateContinue)
		runner.Require.NoError(runner.StartWorker())

		select {
		case result := <-updateResult:
			runner.Require.Equal(result, 0)
		case err := <-updateErr:
			return run, err
		}
		time.Sleep(time.Second)
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
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
	counter := 0
	if err := workflow.SetUpdateHandler(ctx, fetchAndAdd,
		func(ctx workflow.Context, i int) (int, error) {
			actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{ScheduleToCloseTimeout: 10 * time.Second})
			if err := workflow.ExecuteActivity(actx, Block).Get(ctx, nil); err != nil {
				return 0, err
			}
			tmp := counter
			counter += i
			return tmp, nil
		},
	); err != nil {
		return 0, err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return counter, ctx.Err()
}
