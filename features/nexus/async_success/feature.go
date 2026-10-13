package async_success

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

func HandlerWorkflow(ctx workflow.Context, name string) (string, error) {
	return "Hello, " + name + "!", nil
}

var AsyncOperation = temporalnexus.NewWorkflowRunOperation(
	"say-hello-async",
	HandlerWorkflow,
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		return client.StartWorkflowOptions{ID: "async-success-" + name}, nil
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
	var exec workflow.NexusOperationExecution
	if err := fut.GetNexusOperationExecution().Get(ctx, &exec); err != nil {
		return "", err
	}
	if exec.OperationToken == "" {
		return "", harness.AppErrorf("expected a non-empty operation token")
	}
	var result string
	if err := fut.Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{Workflow, HandlerWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "Hello, world!",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		for _, t := range []enumspb.EventType{
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED,
		} {
			hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			ev, err := harness.FindEvent(hist, func(ev *historypb.HistoryEvent) bool { return ev.EventType == t })
			if err != nil {
				return err
			}
			if ev == nil {
				return fmt.Errorf("did not find %v event in history", t)
			}
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
