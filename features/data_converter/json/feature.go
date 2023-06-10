package json

import (
	"context"
	"encoding/json"

	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

type Message struct {
	Spec bool `json:"spec"`
}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
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
	runner.Require.Equal("json/plain", encoding)

	resultInHistory := Message{}
	if err := json.Unmarshal(payload.GetData(), &resultInHistory); err != nil {
		return err
	}
	runner.Require.Equal(result, resultInHistory)

	return nil
}
