package result

import (
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/workflow"
)

const (
	ChildWorkflowInput = "test"
)

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, ChildWorkflow},
	ExpectRunResult: ChildWorkflowInput,
}

func Workflow(ctx workflow.Context) (string, error) {
	cwo := workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 10 * time.Minute,
		WorkflowTaskTimeout:      time.Minute,
	}
	ctx = workflow.WithChildOptions(ctx, cwo)
	var childWorkflowResult string
	err := workflow.ExecuteChildWorkflow(ctx, ChildWorkflow, ChildWorkflowInput).Get(ctx, &childWorkflowResult)
	if err != nil {
		return "", err
	}

	return childWorkflowResult, nil
}

func ChildWorkflow(ctx workflow.Context, parameter string) (string, error) {
	return parameter, nil
}
