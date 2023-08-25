package external

import (
	"context"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	SignalName = "external_signal_channel"
	SignalData = "Signaled!"
)

var Feature = harness.Feature{
	Workflows:       Workflow,
	ExpectRunResult: SignalData,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		err = runner.Client.SignalWorkflow(
			context.Background(),
			run.GetID(),
			run.GetRunID(),
			SignalName,
			SignalData,
		)
		if err != nil {
			return nil, err
		}
		return run, nil
	},
}

// Workflow waits for a single signal and returns the data contained therein
func Workflow(ctx workflow.Context) (string, error) {
	wfResult := ""
	signalCh := workflow.GetSignalChannel(ctx, SignalName)
	signalCh.Receive(ctx, &wfResult)
	return wfResult, nil
}
