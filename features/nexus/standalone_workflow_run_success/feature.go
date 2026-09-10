package standalone_workflow_run_success

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

func HandlerWorkflow(ctx workflow.Context, name string) (string, error) {
	return "Hello, " + name + "!", nil
}

var AsyncWorkflowOperation = temporalnexus.NewWorkflowRunOperation(
	"AsyncWorkflowOperation",
	HandlerWorkflow,
	func(ctx context.Context, input string, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		return client.StartWorkflowOptions{ID: "nexus-standalone-handler-" + opts.RequestID}, nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(AsyncWorkflowOperation)
	return s
}()

var Feature = harness.Feature{
	Workflows:     HandlerWorkflow,
	NexusServices: Service,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		// Start a Nexus operation directly from the client, without a caller workflow.
		nc, err := runner.Client.NewNexusClient(client.NexusClientOptions{
			Endpoint: runner.NexusEndpoint,
			Service:  ServiceName,
		})
		if err != nil {
			return nil, fmt.Errorf("create nexus client: %w", err)
		}
		handle, err := nc.ExecuteOperation(ctx, AsyncWorkflowOperation, "world", client.StartNexusOperationOptions{
			ID:                     "standalone-op-" + uuid.NewString(),
			ScheduleToCloseTimeout: time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("execute operation: %w", err)
		}
		var result string
		if err := handle.Get(ctx, &result); err != nil {
			return nil, fmt.Errorf("get operation result: %w", err)
		}
		if result != "Hello, world!" {
			return nil, fmt.Errorf("expected %q, got %q", "Hello, world!", result)
		}
		// Return nil run so the harness skips the default workflow-run checks.
		return nil, nil
	},
}
