package sync_success

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

var SyncOperation = nexus.NewSyncOperation(
	"say-hello",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		return "Hello, " + name + "!", nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(SyncOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	fut := nc.ExecuteOperation(ctx, SyncOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	var result string
	if err := fut.Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "Hello, world!",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		// Sync Nexus operations should transition directly from Scheduled to Completed with
		// no Started event in between.
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED)
	},
}
