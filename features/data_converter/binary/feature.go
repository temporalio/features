package binary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gogo/protobuf/jsonpb"
	common "go.temporal.io/api/common/v1"
	historyProto "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var EXPECTED_RESULT = []byte{0xde, 0xad, 0xbe, 0xef}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
}

// run a workflow that returns binary value `0xdeadbeef`
func Workflow(ctx workflow.Context) ([]byte, error) {
	return EXPECTED_RESULT, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is binary `0xdeadbeef`
	result := make([]byte, 4)
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	if !bytes.Equal(result, EXPECTED_RESULT) {
		return fmt.Errorf("invalid result: %v", result)
	}
	history := runner.Client.GetWorkflowHistory(ctx, run.GetID(), "", false, 0)

	event, err := harness.FindEvent(history, func(ev *historyProto.HistoryEvent) bool {
		attrs := ev.GetWorkflowExecutionCompletedEventAttributes()
		return attrs != nil
	})
	if err != nil {
		return err
	}

	attrs := event.GetWorkflowExecutionCompletedEventAttributes()
	if attrs == nil {
		return errors.New("could not locate WorkflowExecutionCompleted event")
	}
	payload := attrs.GetResult().GetPayloads()[0]

	// load JSON payload from `./payload.json` and compare it to result payload
	file, err := os.Open(filepath.Join(runner.Feature.AbsDir, "../../../features/data_converter/binary/payload.json"))
	if err != nil {
		return err
	}

	expectedPayload := &common.Payload{}
	unmarshaler := jsonpb.Unmarshaler{}
	err = unmarshaler.Unmarshal(file, expectedPayload)
	if err != nil {
		return err
	}
	runner.Require.Equal(expectedPayload, payload)
	return nil
}
