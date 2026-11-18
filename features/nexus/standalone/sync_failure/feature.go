package sync_failure

import (
	"context"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/features/nexus/standalone"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	failureType    = "StandaloneNexusTestFailure"
	failureMessage = "deliberate sync handler failure"
)

var operation = nexus.NewSyncOperation(
	"sync-failure",
	func(_ context.Context, _ string, _ nexus.StartOperationOptions) (string, error) {
		return "", &nexus.OperationError{
			State:   nexus.OperationStateFailed,
			Message: "sync operation failed",
			Cause:   temporal.NewApplicationError(failureMessage, failureType),
		}
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
		return nil, standalone.CheckApplicationFailure(handle.Get(ctx, nil), failureType, failureMessage)
	},
}
