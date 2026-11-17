package harness

import (
	"context"
	"fmt"
	"reflect"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// CheckNexusOperationError checks the immediate wrapper, not just an error somewhere
// in the cause chain. token is empty for an operation that did not start asynchronously.
func CheckNexusOperationError(err error, endpoint, service, operation, token string) (*temporal.NexusOperationError, error) {
	opErr, ok := err.(*temporal.NexusOperationError)
	if !ok {
		return nil, AppErrorf("expected NexusOperationError, got %T: %v", err, err)
	}
	if opErr.Endpoint != endpoint || opErr.Service != service || opErr.Operation != operation ||
		opErr.OperationToken != token || opErr.ScheduledEventID <= 0 {
		return nil, AppErrorf("unexpected Nexus operation metadata: endpoint=%q service=%q operation=%q token=%q scheduledEventID=%d",
			opErr.Endpoint, opErr.Service, opErr.Operation, opErr.OperationToken, opErr.ScheduledEventID)
	}
	return opErr, nil
}

// CheckNexusApplicationCause checks a leaf application failure and its payload.
func CheckNexusApplicationCause(err error, typ, message, detail string) error {
	appErr, ok := err.(*temporal.ApplicationError)
	if !ok {
		return AppErrorf("expected application cause, got %T: %v", err, err)
	}
	if appErr.Type() != typ || appErr.Message() != message || appErr.NonRetryable() || appErr.Unwrap() != nil {
		return AppErrorf("unexpected application cause: %v", appErr)
	}
	var actual string
	if err := appErr.Details(&actual); err != nil || actual != detail {
		return AppErrorf("expected application detail %q, got %q (%v)", detail, actual, err)
	}
	return nil
}

// CheckNexusTimeoutCause checks the immediate timeout cause, including its type.
func CheckNexusTimeoutCause(err error, timeoutType enumspb.TimeoutType) error {
	timeoutErr, ok := err.(*temporal.TimeoutError)
	if !ok {
		return AppErrorf("expected timeout cause, got %T: %v", err, err)
	}
	if timeoutErr.TimeoutType() != timeoutType || timeoutErr.Unwrap() != nil || timeoutErr.HasLastHeartbeatDetails() {
		return AppErrorf("expected leaf %v timeout without heartbeat details, got %v", timeoutType, timeoutErr)
	}
	return nil
}

// CheckNexusHistory checks the complete Nexus lifecycle of a single operation,
// including the absence of unexpected started or terminal events, then replays it.
// Successful cancellation acknowledgements are omitted because they can race with
// completion of the operation itself.
func CheckNexusHistory(ctx context.Context, runner *Runner, run client.WorkflowRun, expected ...enumspb.EventType) error {
	var actual []enumspb.EventType
	hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	for hist.HasNext() {
		event, err := hist.Next()
		if err != nil {
			return err
		}
		switch event.EventType {
		case enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCELED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUESTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_FAILED:
			actual = append(actual, event.EventType)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("expected Nexus lifecycle %v, got %v", expected, actual)
	}
	return runner.CheckHistoryDefault(ctx, run)
}
