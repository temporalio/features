package sync_operation_error

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ServiceName  = "test-service"
	ErrorType    = "TestFailure"
	ErrorMessage = "deliberate failure"
)

var FailingOperation = nexus.NewSyncOperation(
	"fail",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		return "", &nexus.OperationError{
			State: nexus.OperationStateFailed,
			Cause: temporal.NewApplicationError(ErrorMessage, ErrorType),
		}
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(FailingOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	fut := nc.ExecuteOperation(ctx, FailingOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	err := fut.Get(ctx, nil)
	if err == nil {
		return "", harness.AppErrorf("expected the operation to fail")
	}
	var opErr *temporal.NexusOperationError
	if !errors.As(err, &opErr) {
		return "", harness.AppErrorf("expected a nexus operation error, got %v", err)
	}
	appErr := findApplicationError(err, ErrorType)
	if appErr == nil {
		return "", harness.AppErrorf("expected an application error of type %v in the cause chain, got %v", ErrorType, err)
	}
	return appErr.Type() + ": " + appErr.Message(), nil
}

func findApplicationError(err error, errType string) *temporal.ApplicationError {
	for ; err != nil; err = errors.Unwrap(err) {
		if appErr, ok := err.(*temporal.ApplicationError); ok && appErr.Type() == errType {
			return appErr
		}
	}
	return nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: ErrorType + ": " + ErrorMessage,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		hasEvent := func(t enumspb.EventType) (bool, error) {
			hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			ev, err := harness.FindEvent(hist, func(ev *historypb.HistoryEvent) bool { return ev.EventType == t })
			return ev != nil, err
		}
		if ok, err := hasEvent(enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("did not find NexusOperationFailed event in history")
		}
		if ok, err := hasEvent(enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("unexpected NexusOperationCompleted event for failed operation")
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
