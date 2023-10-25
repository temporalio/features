package json

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

type Message struct {
	Spec bool `json:"spec"`
}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
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
	runner.Require.Equal("json/plain", encoding)

	resultInHistory := Message{}
	if err := json.Unmarshal(payload.GetData(), &resultInHistory); err != nil {
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
