package updateutil

import (
	"context"
	"errors"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

func CheckServerSupportsUpdate(ctx context.Context, c client.Client) string {
	handle, err := c.UpdateWorkflow(ctx, "fake", "also_fake", "__does_not_exist")
	// Newer versions of the sdk will return a permission denied error here
	var denied *serviceerror.PermissionDenied
	var notFound *serviceerror.NotFound
	if errors.As(err, &denied) {
		return "server support for update is disabled; " +
			"set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	}
	// Otherwise we expect a not found error since the workflowID is fake
	if errors.As(err, &notFound) {
		return ""
	}
	err = handle.Get(ctx, nil)

	// A few early versions of the sdk will return the permission denied error
	// here so check for that as well
	if errors.As(err, &denied) {
		return "server support for update is disabled; " +
			"set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	}
	var unimplemented *serviceerror.Unimplemented
	if errors.As(err, &unimplemented) {
		return "server version too old to support update"
	}
	return ""
}
