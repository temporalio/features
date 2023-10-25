package codec

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

// Add custom codec to default data converter
func getCustomConverter() converter.DataConverter {
	return converter.NewCodecDataConverter(
		converter.GetDefaultDataConverter(),
		newBase64Codec(),
	)
}

func decodeBase64(in []byte) ([]byte, error) {
	dst, err := base64.StdEncoding.DecodeString(string(in))
	if err != nil {
		return nil, err
	}
	return dst, nil
}

type Message struct {
	Spec bool `json:"spec"`
}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	ClientOptions: client.Options{
		DataConverter: getCustomConverter(),
	},
	// ExecuteDefault does not support workflow arguments
	Execute: harness.ExecuteWithArgs(Workflow, Message{true}),
}

// An "echo" workflow
func Workflow(ctx workflow.Context, res Message) (Message, error) {
	return res, nil
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
	runner.Require.Equal("my-encoding", encoding)

	extractedData, err := decodeBase64(payload.GetData())
	if err != nil {
		return err
	}

	innerPayload := commonpb.Payload{}
	err = proto.Unmarshal(extractedData, &innerPayload)
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

	payloadArg, err := harness.GetWorkflowArgumentPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	runner.Require.True(proto.Equal(payload, payloadArg))

	return nil
}

type base64Codec struct{}

// A simple codec that encodes binary arrays into Base64.
// The encoding type is "my-encoding", representing an arbitrary custom codec.
func newBase64Codec() converter.PayloadCodec { return &base64Codec{} }

func (b *base64Codec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		b, err := proto.Marshal(p)
		if err != nil {
			return payloads, err
		}
		result[i] = &commonpb.Payload{
			Metadata: map[string][]byte{converter.MetadataEncoding: []byte("my-encoding")},
			Data:     []byte(base64.StdEncoding.EncodeToString(b)),
		}
	}
	return result, nil
}

func (b *base64Codec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		// Only if it's our encoding
		if string(p.Metadata[converter.MetadataEncoding]) != "my-encoding" {
			result[i] = p
			continue
		}
		dst, err := base64.StdEncoding.DecodeString(string(p.Data))
		if err != nil {
			return payloads, err
		}
		result[i] = &commonpb.Payload{}
		err = proto.Unmarshal(dst, result[i])
		if err != nil {
			return payloads, err
		}
	}
	return result, nil
}
