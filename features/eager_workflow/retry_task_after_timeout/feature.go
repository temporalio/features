package retry_task_after_timeout

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

func Workflow(ctx workflow.Context) (string, error) { return "ok", nil }

var Feature = harness.Feature{
	Workflows:            Workflow,
	RequiredCapabilities: &workflowservice.GetSystemInfoResponse_Capabilities{EagerWorkflowStart: true},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		taskTimeout := 2 * time.Second // Should be enough even in slow CI
		workflowID := uuid.NewString()
		response, err := runner.Client.WorkflowService().StartWorkflowExecution(ctx, &workflowservice.StartWorkflowExecutionRequest{
			Namespace:             runner.Namespace,
			Identity:              "features test",
			RequestEagerExecution: true,
			WorkflowId:            workflowID,
			WorkflowType:          &common.WorkflowType{Name: "Workflow"},
			TaskQueue:             &taskqueue.TaskQueue{Name: runner.TaskQueue, Kind: enums.TASK_QUEUE_KIND_NORMAL},
			RequestId:             uuid.NewString(),
			WorkflowTaskTimeout:   &taskTimeout,
		})
		if err != nil {
			return nil, err
		}
		task := response.GetEagerWorkflowTask()
		if task == nil {
			return nil, errors.New("StartWorkflowExecution response did not contain a workflow task")
		}
		// Let it timeout

		run := runner.Client.GetWorkflow(ctx, workflowID, response.GetRunId())
		var result string
		if err := run.Get(ctx, &result); err != nil {
			return nil, err
		}
		runner.Assert.Equal("ok", result)
		return run, nil
	},
}
