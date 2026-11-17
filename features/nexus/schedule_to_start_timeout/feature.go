package schedule_to_start_timeout

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	// No Nexus service is registered: nothing can handle this operation.
	fut := workflow.NewNexusClient(endpoint, "test-service").ExecuteOperation(ctx, "unhandled", nil, workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: 30 * time.Second,
		ScheduleToStartTimeout: 3 * time.Second,
	})
	for _, err := range []error{fut.GetNexusOperationExecution().Get(ctx, nil), fut.Get(ctx, nil)} {
		opErr, err := harness.CheckNexusOperationError(err, endpoint, "test-service", "unhandled", "")
		if err != nil {
			return "", err
		}
		if err := harness.CheckNexusTimeoutCause(opErr.Unwrap(), enumspb.TIMEOUT_TYPE_SCHEDULE_TO_START); err != nil {
			return "", err
		}
	}
	return "timed out as expected", nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	ExpectRunResult: "timed out as expected",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT)
	},
}
