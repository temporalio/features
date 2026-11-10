package external_signal

import (
	"context"
	"time"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	signalName = "external"
	signalData = "signaled"
)

var Feature = harness.Feature{
	Workflows:     []interface{}{Workflow, Receiver},
	ClientOptions: sercontext.ClientOptions(),
	Execute:       Execute,
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

// Workflow signals another running workflow. The signal payload is serialized
// with the target's workflow ID, not this workflow's own ID.
func Workflow(ctx workflow.Context, targetID string) (string, error) {
	err := workflow.SignalExternalWorkflow(ctx, targetID, "", signalName, signalData).Get(ctx, nil)
	return targetID, err
}

func Receiver(ctx workflow.Context) (string, error) {
	var received string
	workflow.GetSignalChannel(ctx, signalName).Receive(ctx, &received)
	return received, nil
}

func receiverID(runner *harness.Runner) string {
	return runner.TaskQueue + "_receiver"
}

func Execute(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
	receiver, err := runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       receiverID(runner),
		TaskQueue:                runner.TaskQueue,
		WorkflowExecutionTimeout: time.Minute,
	}, Receiver)
	if err != nil {
		return nil, err
	}

	run, err := harness.ExecuteWithArgs(Workflow, receiver.GetID())(ctx, runner)
	if err != nil {
		return nil, err
	}

	var received string
	if err := receiver.Get(ctx, &received); err != nil {
		return nil, err
	}
	runner.Require.Equal(signalData, received)
	return run, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(receiverID(runner), result)

	expected := sercontext.WorkflowSignature(runner.Namespace, receiverID(runner))
	runner.Require.NotEqual(sercontext.WorkflowSignature(runner.Namespace, run.GetID()), expected)

	senderEvents, err := sercontext.Events(ctx, runner.Client, run.GetID(), run.GetRunID())
	if err != nil {
		return err
	}
	initiated, err := sercontext.FindEvent(senderEvents, "SignalExternalWorkflowExecutionInitiated", func(e *historypb.HistoryEvent) bool {
		return e.GetSignalExternalWorkflowExecutionInitiatedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(initiated.GetSignalExternalWorkflowExecutionInitiatedEventAttributes().GetInput()))

	receiverEvents, err := sercontext.Events(ctx, runner.Client, receiverID(runner), "")
	if err != nil {
		return err
	}
	signaled, err := sercontext.FindEvent(receiverEvents, "WorkflowExecutionSignaled", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionSignaledEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(signaled.GetWorkflowExecutionSignaledEventAttributes().GetInput()))

	return nil
}
