package binary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/gogo/protobuf/jsonpb"

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

// TODO: deuglify this Go code
// We should probably use some assertion library in the harness, check what the author's intent was
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
	for history.HasNext() {
		ev, err := history.Next()
		if err != nil {
			return err
		}
		// get result payload of WorkflowExecutionCompleted event from workflow history
		attrs := ev.GetWorkflowExecutionCompletedEventAttributes()
		if attrs != nil {
			payload := attrs.GetResult().GetPayloads()[0]
			marshaler := jsonpb.Marshaler{}
			str, err := marshaler.MarshalToString(payload)
			if err != nil {
				return err
			}
			payloadJSON, err := harness.UnmarshalPayload([]byte(str))
			if err != nil {
				return err
			}
			fmt.Printf("%v\n", payloadJSON)
			dirname, err := harness.Dirname()
			if err != nil {
				return err
			}

			// load JSON payload from `./payload.json` and compare it to JSON representation of result payload
			// Note how in this wonderful language this is 10 times as long as the others
			contents, err := os.ReadFile(path.Join(dirname, "../../../features/data_converter/binary/payload.json"))
			if err != nil {
				return err
			}
			expectedPayloadJSON, err := harness.UnmarshalPayload(contents)
			if err != nil {
				return err
			}

			if expectedPayloadJSON.Data != payloadJSON.Data {
				return errors.New("payload data mismatch")
			}
			if len(expectedPayloadJSON.Metadata) != len(payloadJSON.Metadata) {
				return errors.New("payload metadata length mismatch")
			}
			if expectedPayloadJSON.Metadata["encoding"] != payloadJSON.Metadata["encoding"] {
				return errors.New("payload metadata encoding mismatch")
			}
		}
	}
	return nil
}
