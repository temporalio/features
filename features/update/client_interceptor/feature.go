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
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

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

		var result int
		runner.Require.NoError(handle.Get(ctx, &result))

		runner.Require.Equal(result, updateArg+addToUpdateArg)

		runner.Require.NoError(runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdownSignal, nil))
		return run, ctx.Err()
	},
}

func Workflow(ctx workflow.Context) (int, error) {
	counter := 0
	if err := workflow.SetUpdateHandler(ctx, updateName,
		func(ctx workflow.Context, i int) (int, error) {
			counter += i
			return counter, nil
		},
	); err != nil {
		return 0, err
	}

	_ = workflow.GetSignalChannel(ctx, shutdownSignal).Receive(ctx, nil)
	return counter, ctx.Err()
}
