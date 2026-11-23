package async_start_to_close_timeout

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

func BlockingWorkflow(ctx workflow.Context, id string) (string, error) {
	ctx.Done().Receive(ctx, nil)
	return "", ctx.Err()
}

var AsyncOperation = temporalnexus.NewWorkflowRunOperation(
	"block-forever",
	BlockingWorkflow,
	func(ctx context.Context, id string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		return client.StartWorkflowOptions{ID: id, WorkflowExecutionTimeout: time.Minute}, nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(AsyncOperation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	fut := workflow.NewNexusClient(endpoint, ServiceName).ExecuteOperation(ctx, AsyncOperation,
		workflow.GetInfo(ctx).WorkflowExecution.ID+"-handler", workflow.NexusOperationOptions{
			ScheduleToCloseTimeout: 30 * time.Second,
			StartToCloseTimeout:    3 * time.Second,
		})
	var execution workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &execution); err != nil {
		return "", err
	}
	if execution.OperationToken == "" {
		return "", harness.AppErrorf("expected non-empty operation token")
	}
	opErr, err := harness.CheckNexusOperationError(fut.Get(ctx, nil), endpoint, ServiceName, "block-forever", execution.OperationToken)
	if err != nil {
		return "", err
	}
	if err := harness.CheckNexusTimeoutCause(opErr.Unwrap(), enumspb.TIMEOUT_TYPE_START_TO_CLOSE); err != nil {
		return "", err
	}
	return "timed out as expected", nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, BlockingWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "timed out as expected",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckResult: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		// A Nexus timeout does not cancel the backing workflow.
		defer runner.Client.CancelWorkflow(ctx, run.GetID()+"-handler", "")
		return runner.CheckResultDefault(ctx, run)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT)
	},
}
