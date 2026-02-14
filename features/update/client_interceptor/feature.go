package intercept

import (
	"context"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
)

const (
	updateName     = "add"
	updateArg      = 5
	addToUpdateArg = 5
	done           = "done"
	shutdownSignal = "shutdown_signal"
)

type (
	AddOneInterceptorFactory struct {
		interceptor.ClientInterceptorBase
	}

	AddOneInterceptor struct {
		interceptor.ClientOutboundInterceptorBase
	}
)

func (*AddOneInterceptorFactory) InterceptClient(
	next interceptor.ClientOutboundInterceptor,
) interceptor.ClientOutboundInterceptor {
	return &AddOneInterceptor{
		ClientOutboundInterceptorBase: interceptor.ClientOutboundInterceptorBase{
			Next: next,
		},
	}
}

// UpdateWorkflow intercepts outbound workflow update calls made via the sdk
// Client and increments arg0 by `addToUpdateArg`
func (aoi *AddOneInterceptor) UpdateWorkflow(
	ctx context.Context,
	in *interceptor.ClientUpdateWorkflowInput,
) (client.WorkflowUpdateHandle, error) {
	if in.UpdateName == updateName {
		in.Args[0] = in.Args[0].(int) + addToUpdateArg
	}
	return aoi.ClientOutboundInterceptorBase.UpdateWorkflow(ctx, in)
}

var Feature = harness.Feature{
	Workflows: Workflow,
	ClientOptions: client.Options{
		Interceptors: []interceptor.ClientInterceptor{
			&AddOneInterceptorFactory{},
		},
	},
	ExpectRunResult: updateArg + addToUpdateArg,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		runner.Log.Info("Starting client_interceptor update test execution")

		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		runner.Log.Info("Starting workflow execution (client has interceptor)")
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		runner.Log.Info("Workflow started", "workflowID", run.GetID(), "runID", run.GetRunID())

		runner.Log.Info("Sending update (interceptor will modify arg)", "updateName", updateName, "originalArg", updateArg, "interceptorWillAdd", addToUpdateArg, "waitForStage", "Completed")
		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			client.UpdateWorkflowOptions{
				WorkflowID:   run.GetID(),
				RunID:        run.GetRunID(),
				UpdateName:   updateName,
				Args:         []interface{}{updateArg},
				WaitForStage: client.WorkflowUpdateStageCompleted,
			},
		)
		runner.Require.NoError(err)
		runner.Log.Info("Update request returned", "updateID", handle.UpdateID())

		runner.Log.Info("Getting update result")
		var result int
		runner.Require.NoError(handle.Get(ctx, &result))
		runner.Log.Info("Update result received", "result", result, "expectedResult", updateArg+addToUpdateArg)

		runner.Require.Equal(result, updateArg+addToUpdateArg)

		runner.Log.Info("Sending shutdown signal to workflow")
		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		runner.Log.Info("Client_interceptor update test completed successfully")
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) (int, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Workflow started, setting up update handler")

	counter := 0
	if err := workflow.SetUpdateHandler(ctx, updateName,
		func(ctx workflow.Context, i int) (int, error) {
			logger.Info("Update handler invoked (arg modified by interceptor)", "arg", i, "currentCounter", counter)
			counter += i
			logger.Info("Update handler completed", "newCounter", counter)
			return counter, nil
		},
	); err != nil {
		return 0, err
	}
	logger.Info("Update handler registered, waiting for shutdown signal")

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	logger.Info("Shutdown signal received, workflow completing", "finalCounter", counter)
	return counter, ctx.Err()
}
