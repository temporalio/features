package signal

import (
	"context"
	"time"

	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	UnblockSignal  = "unblock-signal"
	UnblockMessage = "unblock"
)

// A workflow that starts a child workflow, unblocks it, and returns the result
// of the child workflow.
func Workflow(ctx workflow.Context) (string, error) {
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 10 * time.Minute,
		WorkflowTaskTimeout:      time.Minute,
	})
	childWorkflowFut := workflow.ExecuteChildWorkflow(ctx, ChildWorkflow)
	childWorkflowFut.SignalChildWorkflow(ctx, UnblockSignal, UnblockMessage)
	result := ""
	err := childWorkflowFut.Get(ctx, &result)
	if err != nil {
		return "", err
	}
	return result, nil
}

// A workflow that waits for a signal and returns the data received.
func ChildWorkflow(ctx workflow.Context) (string, error) {
	unblockMessage := ""
	signalCh := workflow.GetSignalChannel(ctx, UnblockSignal)
	signalCh.Receive(ctx, &unblockMessage)
	return unblockMessage, nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, ChildWorkflow},
	ExpectRunResult: UnblockMessage,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		return run, nil
	},
}
