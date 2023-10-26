package empty

import (
	"context"
	"errors"
	"os"
	"path"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/common/v1"
	historyProto "go.temporal.io/api/history/v1"
	"go.temporal.io/api/temporalproto"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
)

var Feature = harness.Feature{
	Workflows:   Workflow,
	Activities:  Activity,
	CheckResult: CheckResult,
}

// run a workflow that calls an activity with a null parameter.
func Workflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	})

	return workflow.ExecuteActivity(ctx, Activity, nil).Get(ctx, nil)
}

func Activity(ctx context.Context, input *string) error {
	// check the null input is serialized correctly
	if input != nil {
		return temporal.NewNonRetryableApplicationError("Activity input should be nil", "BadResult", nil)
	}
	return nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is null
	var result interface{}
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Nil(result)

	// get result payload of ActivityTaskScheduled event from workflow history
	history := runner.Client.GetWorkflowHistory(ctx, run.GetID(), "", false, 0)
	event, err := harness.FindEvent(history, func(ev *historyProto.HistoryEvent) bool {
		attrs := ev.GetActivityTaskScheduledEventAttributes()
		return attrs != nil
	})
	if err != nil {
		return err
	}

	attrs := event.GetActivityTaskScheduledEventAttributes()
	if attrs == nil {
		return errors.New("could not locate WorkflowExecutionCompleted event")
	}
	// verify the activity input payload
	payload := attrs.GetInput().GetPayloads()[0]

	// load JSON payload from `./payload.json` and compare it to result payload
	file, err := os.Open(path.Join(runner.Feature.AbsDir, "payload.json"))
	if err != nil {
		return err
	}

	expectedPayload := &common.Payload{}
	decoder := temporalproto.NewJSONDecoder(file)
	if err := decoder.Decode(expectedPayload); err != nil {
		return err
	}
	runner.Require.True(proto.Equal(expectedPayload, payload))
	return nil
}
