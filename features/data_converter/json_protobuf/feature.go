package json_protobuf

import (
	"bytes"
	"context"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var expectedResult = commonpb.DataBlob{Data: []byte{0xde, 0xad, 0xbe, 0xef}}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	// No need of a custom data converter, the default one prioritizes
	//  ProtoJSONPayload over ProtoPayload
}

func Workflow(ctx workflow.Context) (commonpb.DataBlob, error) {
	return expectedResult, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is DataBlob `0xdeadbeef`
	result := commonpb.DataBlob{}
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.True(proto.Equal(&expectedResult, &result))

	payload, err := harness.GetWorkflowResultPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	encoding := string(payload.GetMetadata()["encoding"])
	runner.Require.Equal("json/protobuf", encoding)

	messageType := string(payload.GetMetadata()["messageType"])
	runner.Require.Equal("temporal.api.common.v1.DataBlob", messageType)

	resultInHistory := commonpb.DataBlob{}
	readerPayloadData := bytes.NewReader(payload.GetData())
	if err := jsonpb.Unmarshal(readerPayloadData, &resultInHistory); err != nil {
		return err
	}

	runner.Require.True(proto.Equal(&result, &resultInHistory))
	return nil
}
