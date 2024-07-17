package updateutil

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const (
	updateDisabledMsg        = "server support for update is disabled; set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable"
	asyncAcceptedDisabledMsg = "server support for asynchronous (accepted) udpates is disabled; set frontend.enableUpdateWorkflowExecutionAsyncAccepted=true in dynamic config to enable"
	tooOldMsg                = "server version too old to support update"
)

func CheckServerSupportsUpdate(
	ctx context.Context,
	sdkclient client.Client,
) string {
	return checkSupport(
		ctx,
		sdkclient,
		client.WorkflowUpdateStageCompleted,
		updateDisabledMsg,
	)
}

func CheckServerSupportsAsyncAcceptedUpdate(
	ctx context.Context,
	sdkclient client.Client,
) string {
	return checkSupport(
		ctx,
		sdkclient,
		client.WorkflowUpdateStageAccepted,
		asyncAcceptedDisabledMsg,
	)
}

func checkSupport(
	ctx context.Context,
	c client.Client,
	waitForStage client.WorkflowUpdateStage,
	deniedMsg string,
) string {
	var (
		denied        *serviceerror.PermissionDenied
		notFound      *serviceerror.NotFound
		unimplemented *serviceerror.Unimplemented
	)

	handle, err := c.UpdateWorkflow(
		ctx,
		client.UpdateWorkflowOptions{
			UpdateID:     uuid.NewString(),
			WorkflowID:   "__does_not_exist",
			UpdateName:   "__does_not_exist",
			WaitForStage: waitForStage,
		},
	)

	switch {
	case errors.As(err, &denied):
		return deniedMsg
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
		return deniedMsg
	case errors.As(err, &unimplemented):
		return tooOldMsg
	}

	return ""
}
