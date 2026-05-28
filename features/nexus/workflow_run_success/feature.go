package workflow_run_success

import (
	"context"
	"fmt"
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

var AsyncWorkflowOperation = temporalnexus.NewWorkflowRunOperation(
	"AsyncWorkflowOperation",
	HandlerWorkflow,
	func(ctx context.Context, input string, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
		// Use the request ID so retried start requests resolve to the same workflow.
		return client.StartWorkflowOptions{ID: "nexus-async-workflow-" + opts.RequestID}, nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(AsyncWorkflowOperation)
	return s
}()

func CallerWorkflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	fut := nc.ExecuteOperation(ctx, AsyncWorkflowOperation, "world", workflow.NexusOperationOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	var result string
	if err := fut.Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

var Feature = harness.Feature{
	Workflows:       []interface{}{CallerWorkflow, HandlerWorkflow},
	NexusServices:   Service,
	ExpectRunResult: "Hello, world!",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, CallerWorkflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		// Async (workflow-run) Nexus operations should transition Scheduled -> Started -> Completed.
		findCallerEvent := func(t enumspb.EventType) (*historypb.HistoryEvent, error) {
			hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			return harness.FindEvent(hist, func(ev *historypb.HistoryEvent) bool { return ev.EventType == t })
		}
		scheduled, err := findCallerEvent(enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED)
		if err != nil {
			return err
		}
		if scheduled == nil {
			return fmt.Errorf("did not find NexusOperationScheduled event in history")
		}
		started, err := findCallerEvent(enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED)
		if err != nil {
			return err
		}
		if started == nil {
			return fmt.Errorf("did not find NexusOperationStarted event in history")
		}
		if completed, err := findCallerEvent(enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED); err != nil {
			return err
		} else if completed == nil {
			return fmt.Errorf("did not find NexusOperationCompleted event in history")
		}

		// The caller's NexusOperationStarted event must link to the handler workflow's
		// WorkflowExecutionStarted event.
		handlerLink := findWorkflowEventLink(started.GetLinks(), enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED)
		if handlerLink == nil {
			return fmt.Errorf("NexusOperationStarted is missing a link to the handler WorkflowExecutionStarted event")
		}

		// The handler workflow's WorkflowExecutionStarted event carries the Nexus completion
		// callback, whose link points back to the caller's NexusOperationScheduled event.
		// (Nexus links on the started event itself are deduped against the callback link.)
		handlerHist := runner.Client.GetWorkflowHistory(ctx, handlerLink.GetWorkflowId(), handlerLink.GetRunId(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		handlerStarted, err := harness.FindEvent(handlerHist, func(ev *historypb.HistoryEvent) bool {
			return ev.EventType == enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED
		})
		if err != nil {
			return err
		}
		if handlerStarted == nil {
			return fmt.Errorf("did not find WorkflowExecutionStarted event in handler history")
		}
		callbacks := handlerStarted.GetWorkflowExecutionStartedEventAttributes().GetCompletionCallbacks()
		if len(callbacks) == 0 {
			return fmt.Errorf("handler WorkflowExecutionStarted has no completion callbacks")
		}
		callerLink := findWorkflowEventLink(callbacks[0].GetLinks(), enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED)
		if callerLink == nil {
			return fmt.Errorf("handler completion callback is missing a link to the caller NexusOperationScheduled event")
		}
		if callerLink.GetWorkflowId() != run.GetID() || callerLink.GetRunId() != run.GetRunID() {
			return fmt.Errorf("handler callback link references %s/%s, expected caller %s/%s",
				callerLink.GetWorkflowId(), callerLink.GetRunId(), run.GetID(), run.GetRunID())
		}
		return nil
	},
}

// findWorkflowEventLink returns the first WorkflowEvent-variant link whose event reference matches
// the given event type, or nil if none match.
func findWorkflowEventLink(links []*commonpb.Link, eventType enumspb.EventType) *commonpb.Link_WorkflowEvent {
	for _, l := range links {
		we := l.GetWorkflowEvent()
		if we == nil {
			continue
		}
		if we.GetEventRef().GetEventType() == eventType {
			return we
		}
	}
	return nil
}
