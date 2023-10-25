package json_protobuf

import (
	"context"

	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/temporalproto"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var expectedResult = commonpb.DataBlob{Data: []byte{0xde, 0xad, 0xbe, 0xef}}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	// ExecuteDefault does not support workflow arguments
	Execute: harness.ExecuteWithArgs(Workflow, &expectedResult),
	// No need of a custom data converter, the default one prioritizes
	// ProtoJSONPayload over ProtoPayload
}

// An "echo" workflow
func Workflow(ctx workflow.Context, res *commonpb.DataBlob) (*commonpb.DataBlob, error) {
	return res, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is DataBlob `0xdeadbeef`
	result := commonpb.DataBlob{}
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.ProtoRequire.ProtoEqual(&expectedResult, &result)

	payload, err := harness.GetWorkflowResultPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	encoding := string(payload.GetMetadata()["encoding"])
	runner.Require.Equal("json/protobuf", encoding)

	messageType := string(payload.GetMetadata()["messageType"])
	runner.Require.Equal("temporal.api.common.v1.DataBlob", messageType)

	resultInHistory := commonpb.DataBlob{}
	if err := temporalproto.UnmarshalJSON(payload.GetData(), &resultInHistory); err != nil {
		return err
	}

	runner.ProtoRequire.ProtoEqual(&result, &resultInHistory)

	payloadArg, err := harness.GetWorkflowArgumentPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	runner.ProtoRequire.ProtoEqual(payload, payloadArg)

	return nil
}
