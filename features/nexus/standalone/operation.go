package standalone

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

const serviceName = "standalone-nexus"

func NewService(operation nexus.Operation[string, string]) *nexus.Service {
	service := nexus.NewService(serviceName)
	service.MustRegister(operation)
	return service
}

func StartOperation(ctx context.Context, runner *harness.Runner, operation nexus.Operation[string, string], input string) (client.NexusOperationHandle, error) {
	nexusClient, err := runner.Client.NewNexusClient(client.NexusClientOptions{
		Endpoint: runner.NexusEndpoint,
		Service:  serviceName,
	})
	if err != nil {
		return nil, err
	}

	handle, err := nexusClient.ExecuteOperation(ctx, operation, input, client.StartNexusOperationOptions{
		ID:                     operation.Name() + "-" + uuid.NewString(),
		ScheduleToCloseTimeout: time.Minute,
	})
	if err != nil {
		var unimplemented *serviceerror.Unimplemented
		if errors.As(err, &unimplemented) {
			return nil, runner.Skip("server does not implement the standalone Nexus API")
		}
		return nil, fmt.Errorf("start operation %q: %w", operation.Name(), err)
	}
	return handle, nil
}

func BackingWorkflowOptions(_ context.Context, _ string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
	return client.StartWorkflowOptions{ID: options.RequestID}, nil
}

func CheckApplicationFailure(err error, errorType, message string) error {
	// A sync operation's failure can have an OperationError wrapper around the application failure.
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		if appErr, ok := cause.(*temporal.ApplicationError); ok &&
			appErr.Type() == errorType && appErr.Message() == message {
			return nil
		}
	}
	return fmt.Errorf("expected application failure %q (%s), got %v", message, errorType, err)
}
