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
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	})

	var result string
	err := workflow.ExecuteActivity(ctx, Echo).Get(ctx, &result)
	if err != nil {
		return "", err
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 1 * time.Minute,
	})
	err = workflow.ExecuteActivity(ctx, Echo).Get(ctx, &result)
	return result, err
}

func Echo(_ context.Context) (string, error) {
	return "echo", nil
}
