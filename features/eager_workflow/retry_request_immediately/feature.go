package retry_request_immediately

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.temporal.io/api/command/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

// Not used but required by the harness, it will be used when the SDK implements eager workflow dispatch
func Workflow(ctx workflow.Context) (string, error) { return "ok", nil }

var Feature = harness.Feature{
	Workflows:            Workflow,
	RequiredCapabilities: &workflowservice.GetSystemInfoResponse_Capabilities{EagerWorkflowStart: true},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		workflowID := uuid.NewString()
		request := workflowservice.StartWorkflowExecutionRequest{
			Namespace:             runner.Namespace,
			Identity:              "features test",
			RequestEagerExecution: true,
			WorkflowId:            workflowID,
			WorkflowType:          &common.WorkflowType{Name: "Workflow"},
			TaskQueue:             &taskqueue.TaskQueue{Name: runner.TaskQueue, Kind: enums.TASK_QUEUE_KIND_NORMAL},
			RequestId:             uuid.NewString(),
		}

		var task *workflowservice.PollWorkflowTaskQueueResponse
		for i := 0; i < 2; i++ {
			response, err := runner.Client.WorkflowService().StartWorkflowExecution(ctx, &request)
			if err != nil {
				return nil, err
			}
			task = response.GetEagerWorkflowTask()
			if task == nil {
				return nil, errors.New("StartWorkflowExecution response did not contain a workflow task")
			}
		}

		dataConverter := runner.Feature.ClientOptions.DataConverter
		if dataConverter == nil {
			dataConverter = converter.GetDefaultDataConverter()
		}
		payloads, err := dataConverter.ToPayloads("ok")
		if err != nil {
			return nil, err
		}
		completion := workflowservice.RespondWorkflowTaskCompletedRequest{
			Namespace: runner.Namespace,
			Identity:  "features test",
			TaskToken: task.TaskToken,
			Commands: []*command.Command{{CommandType: enums.COMMAND_TYPE_COMPLETE_WORKFLOW_EXECUTION, Attributes: &command.Command_CompleteWorkflowExecutionCommandAttributes{
				CompleteWorkflowExecutionCommandAttributes: &command.CompleteWorkflowExecutionCommandAttributes{
					Result: payloads,
				},
			}}},
		}
		if _, err := runner.Client.WorkflowService().RespondWorkflowTaskCompleted(ctx, &completion); err != nil {
			return nil, err
		}
		run := runner.Client.GetWorkflow(ctx, workflowID, "")
		var result string
		if err := run.Get(ctx, &result); err != nil {
			return nil, err
		}
		runner.Assert.Equal("ok", result)
		return run, nil
	},
}
