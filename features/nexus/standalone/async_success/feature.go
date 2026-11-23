package async_success

import (
	"context"

	"github.com/temporalio/features/features/nexus/standalone"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

func handlerWorkflow(_ workflow.Context, name string) (string, error) {
	return "Hello from the workflow, " + name + "!", nil
}

var operation = temporalnexus.NewWorkflowRunOperation("async-success", handlerWorkflow, standalone.BackingWorkflowOptions)

var Feature = harness.Feature{
	Workflows:     handlerWorkflow,
	NexusServices: standalone.NewService(operation),
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		handle, err := standalone.StartOperation(ctx, runner, operation, "world")
		if err != nil {
			return nil, err
		}

		var result string
		runner.Require.NoError(handle.Get(ctx, &result))
		runner.Require.Equal("Hello from the workflow, world!", result)

		checkLinks(ctx, runner, handle)
		return nil, nil
	},
}

func checkLinks(ctx context.Context, runner *harness.Runner, handle client.NexusOperationHandle) {
	desc, err := handle.Describe(ctx, client.DescribeNexusOperationOptions{})
	runner.Require.NoError(err)
	links := desc.RawInfo.GetLinks()
	runner.Require.Len(links, 1, "expected a link from the operation to the backing workflow")
	workflowLink := links[0].GetWorkflowEvent()
	runner.Require.NotNil(workflowLink)
	runner.Require.Equal(runner.Namespace, workflowLink.GetNamespace())
	runner.Require.NotEmpty(workflowLink.GetWorkflowId())
	runner.Require.NotEmpty(workflowLink.GetRunId())
	runner.Require.Equal(enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED, workflowLink.GetEventRef().GetEventType())

	history := runner.Client.GetWorkflowHistory(ctx, workflowLink.GetWorkflowId(), workflowLink.GetRunId(),
		false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	runner.Require.True(history.HasNext(), "expected history for the linked workflow run")
	started, err := history.Next()
	runner.Require.NoError(err)
	runner.Require.Equal(enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED, started.GetEventType())
	runner.Require.Equal(started.GetEventId(), workflowLink.GetEventRef().GetEventId())

	// The backlink belongs to the Nexus completion callback on the start event.
	callbacks := started.GetWorkflowExecutionStartedEventAttributes().GetCompletionCallbacks()
	runner.Require.Len(callbacks, 1, "expected the standalone operation's completion callback")
	runner.Require.NotNil(callbacks[0].GetNexus())
	links = callbacks[0].GetLinks()
	runner.Require.Len(links, 1, "expected a link back to the standalone operation")
	operationLink := links[0].GetNexusOperation()
	runner.Require.NotNil(operationLink)
	runner.Require.Equal(runner.Namespace, operationLink.GetNamespace())
	runner.Require.Equal(handle.GetID(), operationLink.GetOperationId())
	runner.Require.Equal(handle.GetRunID(), operationLink.GetRunId())
}
