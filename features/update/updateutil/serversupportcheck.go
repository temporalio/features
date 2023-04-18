package updateutil

import (
	"context"
	"errors"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const (
	disabledMsg = "server support for update is disabled; set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	tooOldMsg   = "server version too old to support update"
)

func CheckServerSupportsUpdate(ctx context.Context, c client.Client) string {
	var (
		denied        *serviceerror.PermissionDenied
		notFound      *serviceerror.NotFound
		unimplemented *serviceerror.Unimplemented
	)

	handle, err := c.UpdateWorkflow(ctx, "fake", "also_fake", "__does_not_exist")
	switch {
	case errors.As(err, &denied):
		return disabledMsg
	case errors.As(err, &unimplemented):
		return tooOldMsg
	case errors.As(err, &notFound):
		return ""
	}

	// some older versions of the SDK won't return an error until Handle.Get is
	// called so check here as well
	err = handle.Get(ctx, nil)
	switch {
	case errors.As(err, &denied):
		return disabledMsg
	case errors.As(err, &unimplemented):
		return tooOldMsg
	}

	return ""
}
