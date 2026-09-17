package continue_as_new

import (
	"context"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const finalResult = "done"

var Feature = harness.Feature{
	Workflows:     Workflow,
	ClientOptions: sercontext.ClientOptions(),
	Execute:       harness.ExecuteWithArgs(Workflow, 1),
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context, remaining int) (string, error) {
	if remaining > 0 {
		return "", workflow.NewContinueAsNewError(ctx, Workflow, remaining-1)
	}
	return finalResult, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// GetRunID follows continue-as-new once the run is awaited.
	firstRunID := run.GetRunID()

	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(finalResult, result)

	// Continue-as-new keeps the workflow ID, so both runs share the context.
	expected := sercontext.WorkflowSignature(runner.Namespace, run.GetID())

	firstRunEvents, err := sercontext.Events(ctx, runner.Client, run.GetID(), firstRunID)
	if err != nil {
		return err
	}
	continued, err := sercontext.FindEvent(firstRunEvents, "WorkflowExecutionContinuedAsNew", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionContinuedAsNewEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(continued.GetWorkflowExecutionContinuedAsNewEventAttributes().GetInput()))

	lastRunEvents, err := sercontext.Events(ctx, runner.Client, run.GetID(), "")
	if err != nil {
		return err
	}
	started, err := sercontext.FindEvent(lastRunEvents, "WorkflowExecutionStarted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionStartedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(started.GetWorkflowExecutionStartedEventAttributes().GetInput()))

	completed, err := sercontext.FindEvent(lastRunEvents, "WorkflowExecutionCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(completed.GetWorkflowExecutionCompletedEventAttributes().GetResult()))

	return nil
}
