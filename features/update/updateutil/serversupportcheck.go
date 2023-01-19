package updateutil

import (
	"context"
	"errors"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

func CheckServerSupportsUpdate(ctx context.Context, c client.Client) string {
	handle, _ := c.UpdateWorkflow(ctx, "fake", "also_fake", "__does_not_exist")
	err := handle.Get(ctx, nil)
	var denied *serviceerror.PermissionDenied
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
