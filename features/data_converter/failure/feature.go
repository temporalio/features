package failure

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.temporal.io/api/failure/v1"
	historyProto "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:   Workflow,
	Activities:  FailureActivity,
	CheckResult: CheckResult,
	ClientOptions: client.Options{
		FailureConverter: temporal.NewDefaultFailureConverter(temporal.DefaultFailureConverterOptions{
			EncodeCommonAttributes: true,
		}),
	},
}

// run a workflow that calls an activity that will fail.
func Workflow(ctx workflow.Context) error {
	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 1 * time.Minute,
		HeartbeatTimeout:       5 * time.Second,
		RetryPolicy:            harness.RetryDisabled,
	})
	fut := workflow.ExecuteActivity(actCtx, FailureActivity)

	_ = fut.Get(ctx, nil)
	return nil
}

func FailureActivity(ctx context.Context) error {
	return temporal.NewApplicationErrorWithCause("main error", "customType", errors.New("cause error"))
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	if err := run.Get(ctx, nil); err != nil {
		return err
	}

	history := runner.Client.GetWorkflowHistory(ctx, run.GetID(), "", false, 0)
	event, err := harness.FindEvent(history, func(ev *historyProto.HistoryEvent) bool {
		attrs := ev.GetActivityTaskFailedEventAttributes()
		return attrs != nil
	})
	if err != nil {
		return err
	}

	attrs := event.GetActivityTaskFailedEventAttributes()
	if attrs == nil {
		return errors.New("could not locate ActivityTaskFailedEventAttributes event")
	}
	// Verify the main error is encoded, ApplicationErrors in Go do not have a stack trace.
	checkFailure(runner, attrs.Failure, "main error", "")
	// Verify Cause was also encoded
	checkFailure(runner, attrs.Failure.Cause, "cause error", "")
	return nil
}

func checkFailure(runner *harness.Runner, failure *failure.Failure, message string, stacktrace string) {
	runner.Require.Equal("Encoded failure", failure.Message)
	runner.Require.Equal("", failure.StackTrace)
	runner.Require.Equal("json/plain", string(failure.EncodedAttributes.Metadata["encoding"]))
	data := map[string]string{}
	json.Unmarshal(failure.EncodedAttributes.Data, &data)
	runner.Require.Equal(message, data["message"])
	runner.Require.Equal(stacktrace, data["stack_trace"])
}
