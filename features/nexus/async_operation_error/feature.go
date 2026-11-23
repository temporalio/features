package async_operation_error

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const (
	ServiceName  = "test-service"
	ErrorType    = "TestFailure"
	ErrorMessage = "deliberate workflow failure"
	ErrorDetail  = "application detail"
)

func HandlerWorkflow(ctx workflow.Context, name string) (string, error) {
	return "", temporal.NewApplicationError(ErrorMessage, ErrorType, ErrorDetail)
}

var AsyncOperation = temporalnexus.NewWorkflowRunOperation(
	"fail-async",
	HandlerWorkflow,
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		return client.StartWorkflowOptions{ID: options.RequestID}, nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(AsyncOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	fut := nc.ExecuteOperation(ctx, AsyncOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	var execution workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &execution); err != nil {
		return "", err
	}
	if execution.OperationToken == "" {
		return "", harness.AppErrorf("expected non-empty operation token")
	}
	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, ServiceName, "fail-async", execution.OperationToken)
	if err != nil {
		return "", err
	}
	// A backing workflow's application failure is the immediate cause: there
	// must not be a WorkflowExecutionError or sync OperationError wrapper.
	if err := harness.CheckNexusApplicationCause(opErr.Unwrap(), ErrorType, ErrorMessage, ErrorDetail); err != nil {
		return "", err
	}
	return "failed as expected", nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, HandlerWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "failed as expected",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_FAILED)
	},
}
