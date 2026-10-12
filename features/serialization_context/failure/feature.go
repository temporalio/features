package failure

import (
	"context"
	"time"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	activityErrorMessage = "activity failed"
	workflowErrorMessage = "workflow failed"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Activities:    Activity,
	ClientOptions: sercontext.ClientOptions(),
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

// Workflow lets an activity fail and then fails itself, so that both an
// activity scoped and a workflow scoped failure conversion are recorded.
func Workflow(ctx workflow.Context) error {
	opts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         harness.RetryDisabled,
	}
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), Activity).Get(ctx, nil)
	if err == nil {
		return harness.AppErrorf("expected the activity to fail")
	}
	return temporal.NewApplicationError(workflowErrorMessage, "WorkflowError")
}

func Activity(ctx context.Context) error {
	return temporal.NewNonRetryableApplicationError(activityErrorMessage, "ActivityError", nil)
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	err := run.Get(ctx, nil)
	runner.Require.Error(err)
	runner.Require.Contains(err.Error(), workflowErrorMessage)

	events, err := sercontext.Events(ctx, runner.Client, run.GetID(), run.GetRunID())
	if err != nil {
		return err
	}

	started, err := sercontext.FindEvent(events, "WorkflowExecutionStarted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionStartedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	scheduled, err := sercontext.FindEvent(events, "ActivityTaskScheduled", func(e *historypb.HistoryEvent) bool {
		return e.GetActivityTaskScheduledEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	scheduledAttrs := scheduled.GetActivityTaskScheduledEventAttributes()

	activityFailed, err := sercontext.FindEvent(events, "ActivityTaskFailed", func(e *historypb.HistoryEvent) bool {
		return e.GetActivityTaskFailedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(
		sercontext.ActivitySignature(
			runner.Namespace,
			run.GetID(),
			started.GetWorkflowExecutionStartedEventAttributes().GetWorkflowType().GetName(),
			scheduledAttrs.GetActivityType().GetName(),
			scheduledAttrs.GetTaskQueue().GetName(),
			false,
		),
		activityFailed.GetActivityTaskFailedEventAttributes().GetFailure().GetSource(),
	)

	workflowFailed, err := sercontext.FindEvent(events, "WorkflowExecutionFailed", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionFailedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(
		sercontext.WorkflowSignature(runner.Namespace, run.GetID()),
		workflowFailed.GetWorkflowExecutionFailedEventAttributes().GetFailure().GetSource(),
	)

	return nil
}
