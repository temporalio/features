package activity_payloads

import (
	"context"
	"time"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	workflowInput = "hello"
	heartbeatData = "beat"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Activities:    Activity,
	ClientOptions: sercontext.ClientOptions(),
	Execute:       harness.ExecuteWithArgs(Workflow, workflowInput),
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context, input string) (string, error) {
	opts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Millisecond, MaximumAttempts: 2},
	}
	var result string
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), Activity, input).Get(ctx, &result)
	return result, err
}

// Activity heartbeats and fails on its first attempt so that its second attempt
// has to decode the heartbeat details recorded by the first one.
func Activity(ctx context.Context, input string) (string, error) {
	if activity.GetInfo(ctx).Attempt == 1 {
		activity.RecordHeartbeat(ctx, heartbeatData)
		return "", harness.AppErrorf("retrying to read back heartbeat details")
	}
	var details string
	if err := activity.GetHeartbeatDetails(ctx, &details); err != nil {
		return "", err
	}
	return input + "|" + details, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(workflowInput+"|"+heartbeatData, result)

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
	expected := sercontext.ActivitySignature(
		runner.Namespace,
		run.GetID(),
		started.GetWorkflowExecutionStartedEventAttributes().GetWorkflowType().GetName(),
		scheduledAttrs.GetActivityType().GetName(),
		scheduledAttrs.GetTaskQueue().GetName(),
		false,
	)
	runner.Require.Equal(expected, sercontext.FirstSignature(scheduledAttrs.GetInput()))

	completed, err := sercontext.FindEvent(events, "ActivityTaskCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetActivityTaskCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(expected,
		sercontext.FirstSignature(completed.GetActivityTaskCompletedEventAttributes().GetResult()))

	workflowCompleted, err := sercontext.FindEvent(events, "WorkflowExecutionCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetWorkflowExecutionCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(
		sercontext.WorkflowSignature(runner.Namespace, run.GetID()),
		sercontext.FirstSignature(workflowCompleted.GetWorkflowExecutionCompletedEventAttributes().GetResult()),
	)

	return nil
}
