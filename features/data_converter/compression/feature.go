package compression

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"

	"github.com/gogo/protobuf/proto"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

// Add Zlib compression codec to default data converter
func getCompressionConverter() converter.DataConverter {
	return converter.NewCodecDataConverter(
		converter.GetDefaultDataConverter(),
		converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
	)
}

func unzip(in []byte) ([]byte, error) {
	rIn, err := zlib.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer rIn.Close()

	return io.ReadAll(rIn)
}

type Message struct {
	Spec bool `json:"spec"`
}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	ClientOptions: client.Options{
		DataConverter: getCompressionConverter(),
	},
}

func Workflow(ctx workflow.Context) (Message, error) {
	return Message{true}, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is `{"spec": true}`
	result := Message{}
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(Message{true}, result)

	payload, err := harness.GetWorkflowResultPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	var encoding = string(payload.GetMetadata()["encoding"])
	runner.Require.Equal("binary/zlib", encoding)

	unzippedData, err := unzip(payload.GetData())
	if err != nil {
		return err
	}

	innerPayload := commonpb.Payload{}
	err = proto.Unmarshal(unzippedData, &innerPayload)
	if err != nil {
		return err
	}

	encoding = string(innerPayload.GetMetadata()["encoding"])
	runner.Require.Equal("json/plain", encoding)

	resultInHistory := Message{}
	if err := json.Unmarshal(innerPayload.GetData(), &resultInHistory); err != nil {
		return err
	}
	runner.Require.Equal(result, resultInHistory)

	return nil
}
