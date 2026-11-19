package sync_handler_error_non_retryable

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
	handlerMessage = "standalone handler rejected the request"
	causeType      = "StandaloneHandlerCause"
	causeMessage   = "rejection from the handler"
	causeDetail    = "standalone handler error detail"
)

var operation = nexus.NewSyncOperation(
	"sync-handler-error",
	func(_ context.Context, _ string, _ nexus.StartOperationOptions) (string, error) {
		return "", &nexus.HandlerError{
			Type:          nexus.HandlerErrorTypeUnavailable,
			Message:       handlerMessage,
			RetryBehavior: nexus.HandlerErrorRetryBehaviorNonRetryable,
			Cause:         temporal.NewApplicationError(causeMessage, causeType, causeDetail),
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

		var handlerErr *nexus.HandlerError
		runner.Require.ErrorAs(handle.Get(ctx, nil), &handlerErr)
		runner.Require.Equal(nexus.HandlerErrorTypeUnavailable, handlerErr.Type)
		runner.Require.Equal(handlerMessage, handlerErr.Message)
		runner.Require.Equal(nexus.HandlerErrorRetryBehaviorNonRetryable, handlerErr.RetryBehavior)
		runner.Require.False(handlerErr.Retryable())
		return nil, harness.CheckNexusApplicationCause(handlerErr.Unwrap(), causeType, causeMessage, causeDetail)
	},
}
