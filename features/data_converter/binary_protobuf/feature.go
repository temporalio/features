package binary_protobuf

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gogo/protobuf/proto"
	pb "go.temporal.io/features/features/data_converter"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

var EXPECTED_RESULT = []byte{0xde, 0xad, 0xbe, 0xef}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	ClientOptions: client.Options{
		DataConverter: converter.NewCompositeDataConverter(
			converter.NewNilPayloadConverter(),
			// Disable ByteSlice and ProtoJSON converters
			converter.NewProtoPayloadConverter(),
			converter.NewJSONPayloadConverter(),
		),
	},
}

func Workflow(ctx workflow.Context) (pb.BinaryMessage, error) {
	return pb.BinaryMessage{Data: EXPECTED_RESULT}, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is BinaryMessage `0xdeadbeef`
	result := pb.BinaryMessage{}
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	if !bytes.Equal(result.Data, EXPECTED_RESULT) {
		return fmt.Errorf("invalid result: %v", result)
	}

	payload, err := harness.GetWorkflowResultPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	var encoding = string(payload.GetMetadata()["encoding"])
	runner.Require.Equal("binary/protobuf", encoding)

	resultInHistory := pb.BinaryMessage{}
	if err := proto.Unmarshal(payload.GetData(), &resultInHistory); err != nil {
		return err
	}

	if !bytes.Equal(resultInHistory.GetData(), EXPECTED_RESULT) {
		return fmt.Errorf("invalid result in history: %v", resultInHistory)
	}

	runner.Require.Equal(result, resultInHistory)
	return nil
}
