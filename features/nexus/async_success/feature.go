package async_success

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
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
	CheckHistory: checkHistory,
}

func checkHistory(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	if err := harness.CheckNexusHistory(ctx, runner, run,
		enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
		enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED,
		enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED); err != nil {
		return err
	}

	var scheduled, started *historypb.HistoryEvent
	callerHistory := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	for callerHistory.HasNext() {
		event, err := callerHistory.Next()
		if err != nil {
			return err
		}
		switch event.GetEventType() {
		case enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED:
			scheduled = event
		case enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED:
			started = event
		}
	}
	if scheduled == nil || started == nil {
		return harness.AppErrorf("expected scheduled and started events in caller history")
	}

	// The handler uses the operation request ID as its workflow ID. Resolve the
	// backing run independently of the link under test.
	handlerID := scheduled.GetNexusOperationScheduledEventAttributes().GetRequestId()
	if handlerID == "" {
		return harness.AppErrorf("expected a request ID on the NexusOperationScheduled event")
	}
	description, err := runner.Client.DescribeWorkflowExecution(ctx, handlerID, "")
	if err != nil {
		return err
	}
	handler := description.GetWorkflowExecutionInfo().GetExecution()
	handlerHistory := runner.Client.GetWorkflowHistory(ctx, handler.GetWorkflowId(), handler.GetRunId(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	if !handlerHistory.HasNext() {
		return harness.AppErrorf("expected history for backing workflow %q", handlerID)
	}
	handlerStarted, err := handlerHistory.Next()
	if err != nil {
		return err
	}
	if handlerStarted.GetEventType() != enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED {
		return harness.AppErrorf("expected backing history to begin with WorkflowExecutionStarted, got %v", handlerStarted.GetEventType())
	}

	if err := checkWorkflowLink("NexusOperationStarted", started.GetLinks(), runner.Namespace, handler, handlerStarted); err != nil {
		return err
	}

	// Servers that deduplicate Nexus callback links expose the backlink on
	// the completion callback instead of on WorkflowExecutionStarted.
	handlerLinks := append([]*commonpb.Link(nil), handlerStarted.GetLinks()...)
	for _, info := range description.GetCallbacks() {
		if callback := info.GetCallback(); callback.GetNexus() != nil {
			handlerLinks = append(handlerLinks, callback.GetLinks()...)
		}
	}
	caller := &commonpb.WorkflowExecution{WorkflowId: run.GetID(), RunId: run.GetRunID()}
	return checkWorkflowLink("backing workflow start or Nexus completion callback", handlerLinks, runner.Namespace, caller, scheduled)
}

func checkWorkflowLink(source string, links []*commonpb.Link, namespace string, target *commonpb.WorkflowExecution, targetEvent *historypb.HistoryEvent) error {
	for _, link := range links {
		workflowEvent := link.GetWorkflowEvent()
		ref := workflowEvent.GetEventRef()
		if workflowEvent.GetNamespace() == namespace &&
			workflowEvent.GetWorkflowId() == target.GetWorkflowId() &&
			workflowEvent.GetRunId() == target.GetRunId() &&
			ref.GetEventType() == targetEvent.GetEventType() &&
			// A link to the workflow's unique start event may omit its event ID.
			(ref.GetEventId() == targetEvent.GetEventId() ||
				(ref.GetEventId() == 0 && targetEvent.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED)) {
			return nil
		}
	}
	return harness.AppErrorf("expected %s to link to %v event %d in %s/%s/%s, got %v",
		source, targetEvent.GetEventType(), targetEvent.GetEventId(),
		namespace, target.GetWorkflowId(), target.GetRunId(), links)
}
