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
	var unimplemented *serviceerror.Unimplemented
	switch {
	case errors.As(err, &denied):
		return "server support for update is disabled; " +
			"set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	case errors.As(err, &unimplemented):
		return "server does not implement the UpdateWorkflow rpc call"
	case errors.As(err, &notFound):
		// expected since the workflow-id does not exist
		return ""
	}

	// A few early versions of the sdk will return the permission denied error
	// here so check for that as well
	err = handle.Get(ctx, nil)
	if errors.As(err, &denied) {
		return "server support for update is disabled; " +
			"set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	}
	return ""
}
