package updateutil

import (
	"context"

	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const shutdown_signal = "shutdown_signal"

func AwaitShutdown(ctx workflow.Context) {
	_ = workflow.GetSignalChannel(ctx, shutdown_signal).Receive(ctx, nil)
}

func RequestShutdown(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) {
	runner.Require.NoError(
		runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), shutdown_signal, nil),
	)
}
