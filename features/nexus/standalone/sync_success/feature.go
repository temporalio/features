package sync_success

import (
	"context"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/features/nexus/standalone"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var operation = nexus.NewSyncOperation(
	"sync-success",
	func(_ context.Context, name string, _ nexus.StartOperationOptions) (string, error) {
		return "Hello from the handler, " + name + "!", nil
	},
)

// The Go harness uses this workflow to locate the feature; the test does not run it.
func workflowPlaceholder(workflow.Context) error { return nil }

var Feature = harness.Feature{
	Workflows:     workflowPlaceholder,
	NexusServices: standalone.NewService(operation),
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		handle, err := standalone.StartOperation(ctx, runner, operation, "world")
		if err != nil {
			return nil, err
		}

		var result string
		runner.Require.NoError(handle.Get(ctx, &result))
		runner.Require.Equal("Hello from the handler, world!", result)
		return nil, nil
	},
}
