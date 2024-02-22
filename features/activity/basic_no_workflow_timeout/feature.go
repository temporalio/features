package retry_on_error

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: Echo,
	StartWorkflowOptionsMutator: func(o *client.StartWorkflowOptions) {
		o.WorkflowExecutionTimeout = 0
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	// Allow 4 retries with no backoff
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	})

	// Execute activity and return error
	var result string
	err := workflow.ExecuteActivity(ctx, Echo).Get(ctx, &result)
	return result, err
}

func Echo(_ context.Context) (string, error) {
	return "echo", nil
}
