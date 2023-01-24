package retry_request_after_timeout

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

// Not used but required by the harness, it will be used when the SDK implements eager workflow dispatch
func Workflow(ctx workflow.Context) (string, error) { return "ok", nil }

var Feature = harness.Feature{
	Workflows:            Workflow,
	RequiredCapabilities: &workflowservice.GetSystemInfoResponse_Capabilities{EagerWorkflowStart: true},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		workflowTaskTimeout := 2 * time.Second
		workflowID := uuid.NewString()
		request := workflowservice.StartWorkflowExecutionRequest{
			Namespace:             runner.Namespace,
			Identity:              "features test",
			RequestEagerExecution: true,
			WorkflowId:            workflowID,
			WorkflowType:          &common.WorkflowType{Name: "Workflow"},
			TaskQueue:             &taskqueue.TaskQueue{Name: runner.TaskQueue, Kind: enums.TASK_QUEUE_KIND_NORMAL},
			RequestId:             uuid.NewString(),
			WorkflowTaskTimeout:   &workflowTaskTimeout,
		}

		var task *workflowservice.PollWorkflowTaskQueueResponse
		response, err := runner.Client.WorkflowService().StartWorkflowExecution(ctx, &request)
		if err != nil {
			return nil, err
		}
		task = response.GetEagerWorkflowTask()
		if task == nil {
			return nil, errors.New("StartWorkflowExecution response did not contain a workflow task")
		}
		time.Sleep(workflowTaskTimeout)
		_, err = runner.Client.WorkflowService().StartWorkflowExecution(ctx, &request)
		runner.Assert.NotNil(err)

		run := runner.Client.GetWorkflow(ctx, workflowID, "")
		var result string
		if err := run.Get(ctx, &result); err != nil {
			return nil, err
		}
		runner.Assert.Equal("ok", result)
		return run, nil
	},
}
