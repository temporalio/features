package standalone

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: Echo,
	Execute:    Execute,
}

func Workflow(workflow.Context) error { return nil }

func Echo(_ context.Context, value string) (string, error) {
	return value, nil
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	handle, err := r.Client.ExecuteActivity(ctx, client.StartActivityOptions{
		ID:                  uuid.NewString(),
		TaskQueue:           r.TaskQueue,
		StartToCloseTimeout: 1 * time.Minute,
	}, Echo, "hello")
	r.Require.NoError(err)

	var result string
	r.Require.NoError(handle.Get(ctx, &result))
	r.Require.Equal("hello", result)

	desc, err := handle.Describe(ctx, client.DescribeActivityOptions{})
	r.Require.NoError(err)
	r.Require.Equal(enums.ACTIVITY_EXECUTION_STATUS_COMPLETED, desc.Status)
	return nil, nil
}
