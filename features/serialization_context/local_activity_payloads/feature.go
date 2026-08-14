package local_activity_payloads

import (
	"context"
	"encoding/json"
	"time"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const workflowInput = "hello"

var Feature = harness.Feature{
	Workflows:     Workflow,
	Activities:    LocalActivity,
	ClientOptions: sercontext.ClientOptions(),
	Execute:       harness.ExecuteWithArgs(Workflow, workflowInput),
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context, input string) (string, error) {
	opts := workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Second}
	var result string
	err := workflow.ExecuteLocalActivity(
		workflow.WithLocalActivityOptions(ctx, opts), LocalActivity, input).Get(ctx, &result)
	return result, err
}

func LocalActivity(ctx context.Context, input string) (string, error) {
	return input + "|local", nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(workflowInput+"|local", result)

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
	startedAttrs := started.GetWorkflowExecutionStartedEventAttributes()

	marker, err := sercontext.FindEvent(events, "LocalActivity marker", func(e *historypb.HistoryEvent) bool {
		return e.GetMarkerRecordedEventAttributes().GetMarkerName() == "LocalActivity"
	})
	if err != nil {
		return err
	}
	details := marker.GetMarkerRecordedEventAttributes().GetDetails()

	// The marker bookkeeping itself belongs to the workflow, its payload carries
	// the workflow context.
	markerData := details["data"].GetPayloads()[0]
	runner.Require.Equal(
		sercontext.WorkflowSignature(runner.Namespace, run.GetID()),
		sercontext.SignatureOf(markerData),
	)

	var decodedMarker struct {
		ActivityType string
	}
	if err := json.Unmarshal(markerData.GetData(), &decodedMarker); err != nil {
		return err
	}

	// The local activity result carries the activity context with IsLocal set.
	runner.Require.Equal(
		sercontext.ActivitySignature(
			runner.Namespace,
			run.GetID(),
			startedAttrs.GetWorkflowType().GetName(),
			decodedMarker.ActivityType,
			startedAttrs.GetTaskQueue().GetName(),
			true,
		),
		sercontext.FirstSignature(details["result"]),
	)

	return nil
}
