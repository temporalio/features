package updateutil

import (
	"context"
	"errors"

	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	updatepb "go.temporal.io/api/update/v1"
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
		enumspb.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_COMPLETED,
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
		enumspb.UPDATE_WORKFLOW_EXECUTION_LIFECYCLE_STAGE_ACCEPTED,
		asyncAcceptedDisabledMsg,
	)
}

func checkSupport(
	ctx context.Context,
	c client.Client,
	stage enumspb.UpdateWorkflowExecutionLifecycleStage,
	deniedMsg string,
) string {
	var (
		denied        *serviceerror.PermissionDenied
		notFound      *serviceerror.NotFound
		unimplemented *serviceerror.Unimplemented
	)

	handle, err := c.UpdateWorkflowWithOptions(
		ctx,
		&client.UpdateWorkflowWithOptionsRequest{
			UpdateID:   uuid.NewString(),
			WorkflowID: "__does_not_exist",
			UpdateName: "__does_not_exist",
			WaitPolicy: &updatepb.WaitPolicy{LifecycleStage: stage},
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
