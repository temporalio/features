package reset_and_delete

import (
	"context"
	"errors"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	historyProto "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	SignalName = "drivesignal"
)

var Feature = harness.Feature{
	Workflows:   Workflow,
	Execute:     Execute,
	CheckResult: CheckResult,
	// We do this kinda by hand in check result -- it's easier since we want to pass the new run
	// to check result, but end up looking at the history of the original run
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		return nil
	},
}

var originalRunID string

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Start the workflow
	run, err := r.ExecuteDefault(ctx)
	originalRunID = run.GetRunID()

	if err != nil {
		return nil, err
	}
	// Drive it for a bit
	for i := 0; i < 5; i++ {
		err := r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), SignalName, "")
		if err != nil {
			return nil, err
		}
	}

	time.Sleep(1 * time.Second)
	// Find event ID of second WFT complete
	iter := r.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	foundID, err := findSecondWFTComplete(iter)
	if err != nil {
		return run, err
	}

	// Reset it
	r.Log.Info("Resetting workflow", "ToEventId", foundID)
	_, err = r.Client.ResetWorkflowExecution(ctx, &workflowservice.ResetWorkflowExecutionRequest{
		Namespace: r.Namespace,
		WorkflowExecution: &common.WorkflowExecution{
			WorkflowId: run.GetID(),
			RunId:      run.GetRunID(),
		},
		Reason:                    "because I feel like it",
		WorkflowTaskFinishEventId: foundID,
	})
	if err != nil {
		return nil, err
	}

	// Drive then finish the now-reset workflow
	newRun := r.Client.GetWorkflow(ctx, run.GetID(), "")
	err = r.Client.SignalWorkflow(ctx, newRun.GetID(), "", SignalName, "")
	if err != nil {
		return nil, err
	}
	err = r.Client.SignalWorkflow(ctx, newRun.GetID(), "", SignalName, "finish")
	if err != nil {
		return nil, err
	}

	return newRun, nil
}

func CheckResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// The reset workflow should have finished normally
	err := r.CheckResultDefault(ctx, run)
	if err != nil {
		return err
	}

	// Now delete it. This can fail initially with "workflow state is not ready"
	err = r.DoUntilEventually(ctx, 1*time.Second, 1*time.Minute, func() bool {
		_, err = r.Client.WorkflowService().DeleteWorkflowExecution(ctx, &workflowservice.DeleteWorkflowExecutionRequest{
			Namespace:         r.Namespace,
			WorkflowExecution: &common.WorkflowExecution{WorkflowId: run.GetID(), RunId: run.GetRunID()},
		})
		return err == nil
	})
	// Make sure it's actually gone, since deletion is async
	err = r.DoUntilEventually(ctx, 1*time.Second, 3*time.Minute, func() bool {
		_, err := r.Client.DescribeWorkflowExecution(ctx, run.GetID(), run.GetRunID())
		var notFoundErr *serviceerror.NotFound
		return errors.As(err, &notFoundErr)
	})
	if err != nil {
		return err
	}

	// Verify original run can still be interacted with
	origRun := r.Client.GetWorkflow(ctx, run.GetID(), originalRunID)
	r.Log.Info("Checking history of original run", "RunID", origRun.GetRunID())
	history := r.Client.GetWorkflowHistory(ctx, origRun.GetID(), origRun.GetRunID(), false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	var next *historyProto.HistoryEvent
	for history.HasNext() {
		next, err = history.Next()
		if err != nil {
			break
		}
	}
	if next.GetEventType() != enums.EVENT_TYPE_WORKFLOW_EXECUTION_TERMINATED {
		return errors.New("expected last event to be terminated")
	}
	// Use eventually since visibility is eventually consistent.
	r.Require.EventuallyWithT(func(t *assert.CollectT) {
		// Ensure original run is findable via visibility & has correct status
		resp, err := r.Client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace: r.Namespace,
			Query:     "WorkflowId = '" + origRun.GetID() + "'",
		})
		assert.NoError(t, err)
		if err != nil {
			return
		}
		assert.Len(t, resp.GetExecutions(), 1)
		if len(resp.GetExecutions()) != 1 {
			return
		}
		assert.Equal(t, enums.WORKFLOW_EXECUTION_STATUS_TERMINATED, resp.GetExecutions()[0].Status)
	}, 200*time.Millisecond, 10*time.Second)
	return nil
}

// Workflow waits for a single signal and returns the data contained therein
func Workflow(ctx workflow.Context) (string, error) {
	signalCh := workflow.GetSignalChannel(ctx, SignalName)
	for {
		var stringResult string
		signalCh.Receive(ctx, &stringResult)
		if stringResult != "" {
			break
		}
	}
	return "", nil
}

func findSecondWFTComplete(iter client.HistoryEventIterator) (int64, error) {
	wftCount := 0
	foundID := int64(0)
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return -1, err
		}
		if event.GetEventType() == enums.EVENT_TYPE_WORKFLOW_TASK_COMPLETED {
			wftCount++
			if wftCount == 2 {
				foundID = event.GetEventId()
				break
			}
		}
	}
	return foundID, nil
}
