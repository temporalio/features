package child_workflow_payloads

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
	workflowInput  = "hello"
	childIDSuffix  = "_child"
	childResultTag = "|child"
)

var Feature = harness.Feature{
	Workflows:     []interface{}{Workflow, ChildWorkflow},
	ClientOptions: sercontext.ClientOptions(),
	Execute:       harness.ExecuteWithArgs(Workflow, workflowInput),
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context, input string) (string, error) {
	opts := workflow.ChildWorkflowOptions{
		WorkflowID:         workflow.GetInfo(ctx).WorkflowExecution.ID + childIDSuffix,
		WorkflowRunTimeout: time.Minute,
	}
	var result string
	err := workflow.ExecuteChildWorkflow(
		workflow.WithChildOptions(ctx, opts), ChildWorkflow, input).Get(ctx, &result)
	return result, err
}

func ChildWorkflow(ctx workflow.Context, input string) (string, error) {
	return input + childResultTag, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(workflowInput+childResultTag, result)

	childID := run.GetID() + childIDSuffix
	// The child's payloads carry the child's own workflow ID, not the parent's.
	expected := sercontext.WorkflowSignature(runner.Namespace, childID)
	runner.Require.NotEqual(sercontext.WorkflowSignature(runner.Namespace, run.GetID()), expected)

	parentEvents, err := sercontext.Events(ctx, runner.Client, run.GetID(), run.GetRunID())
	if err != nil {
		return err
	}

	initiated, err := sercontext.FindEvent(parentEvents, "StartChildWorkflowExecutionInitiated", func(e *historypb.HistoryEvent) bool {
		return e.GetStartChildWorkflowExecutionInitiatedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(initiated.GetStartChildWorkflowExecutionInitiatedEventAttributes().GetInput()))

	childCompleted, err := sercontext.FindEvent(parentEvents, "ChildWorkflowExecutionCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetChildWorkflowExecutionCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(childCompleted.GetChildWorkflowExecutionCompletedEventAttributes().GetResult()))

	childEvents, err := sercontext.Events(ctx, runner.Client, childID, "")
	if err != nil {
		return err
	}
	childStarted, err := sercontext.FindEvent(childEvents, "WorkflowExecutionStarted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionStartedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(childStarted.GetWorkflowExecutionStartedEventAttributes().GetInput()))

	return nil
}
