package async_failure

import (
	"context"

	"github.com/temporalio/features/features/nexus/standalone"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const (
	failureType    = "StandaloneNexusTestFailure"
	failureMessage = "deliberate backing workflow failure"
)

func handlerWorkflow(workflow.Context, string) (string, error) {
	return "", temporal.NewApplicationError(failureMessage, failureType)
}

var operation = temporalnexus.NewWorkflowRunOperation("async-failure", handlerWorkflow, standalone.BackingWorkflowOptions)

var Feature = harness.Feature{
	Workflows:     handlerWorkflow,
	NexusServices: standalone.NewService(operation),
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		handle, err := standalone.StartOperation(ctx, runner, operation, "world")
		if err != nil {
			return nil, err
		}
		return nil, standalone.CheckApplicationFailure(handle.Get(ctx, nil), failureType, failureMessage)
	},
}
