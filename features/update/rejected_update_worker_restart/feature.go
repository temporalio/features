package rejected_update_worker_restart

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
)

const updateName = "add"
const finishSignal = "finish"

var Feature = harness.Feature{
	Workflows:       Workflow,
	ExpectRunResult: 1,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		_, err = runner.WaitForEvent(ctx, run, func(event *historypb.HistoryEvent) bool {
			return event.GetWorkflowTaskCompletedEventAttributes() != nil
		}, 10*time.Second)
		if err != nil {
			return run, err
		}
		before, err := fetchHistory(ctx, runner, run)
		if err != nil {
			return run, err
		}

		if _, err = sendUpdate(ctx, runner, run, -1); err == nil {
			return run, fmt.Errorf("negative Update was accepted")
		}
		after, err := fetchHistory(ctx, runner, run)
		if err != nil {
			return run, err
		}
		if len(before) != len(after) {
			return run, fmt.Errorf("rejected Update changed history length from %d to %d", len(before), len(after))
		}
		for i := range before {
			if !proto.Equal(before[i], after[i]) {
				return run, fmt.Errorf("rejected Update changed history event %d", i+1)
			}
		}

		runner.StopWorker()
		if err = runner.ResetStickyQueue(ctx, run); err != nil {
			return run, err
		}
		if err = runner.StartWorker(); err != nil {
			return run, err
		}
		result, err := sendUpdate(ctx, runner, run, 1)
		if err != nil {
			return run, err
		}
		if result != 1 {
			return run, fmt.Errorf("accepted Update returned %d, expected 1", result)
		}
		return run, runner.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), finishSignal, nil)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		if err := runner.CheckHistoryDefault(ctx, run); err != nil {
			return err
		}
		events, err := fetchHistory(ctx, runner, run)
		if err != nil {
			return err
		}
		var accepted, completed int
		for _, event := range events {
			if event.GetWorkflowExecutionUpdateAcceptedEventAttributes() != nil {
				accepted++
			}
			if event.GetWorkflowExecutionUpdateCompletedEventAttributes() != nil {
				completed++
			}
			if event.GetWorkflowExecutionUpdateRejectedEventAttributes() != nil {
				return fmt.Errorf("rejected Update was recorded in history")
			}
		}
		if accepted != 1 || completed != 1 {
			return fmt.Errorf("expected one accepted and completed Update, got %d accepted and %d completed", accepted, completed)
		}
		return nil
	},
}

func sendUpdate(ctx context.Context, runner *harness.Runner, run client.WorkflowRun, value int) (int, error) {
	handle, err := runner.Client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   run.GetID(),
		RunID:        run.GetRunID(),
		UpdateName:   updateName,
		Args:         []interface{}{value},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		return 0, err
	}
	var result int
	err = handle.Get(ctx, &result)
	return result, err
}

func fetchHistory(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) ([]*historypb.HistoryEvent, error) {
	iter := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false,
		enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	var events []*historypb.HistoryEvent
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func Workflow(ctx workflow.Context) (int, error) {
	counter := 0
	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateName,
		func(_ workflow.Context, value int) (int, error) {
			counter += value
			return counter, nil
		},
		workflow.UpdateHandlerOptions{Validator: func(value int) error {
			if value < 0 {
				return fmt.Errorf("negative value: %d", value)
			}
			return nil
		}},
	); err != nil {
		return 0, err
	}
	workflow.GetSignalChannel(ctx, finishSignal).Receive(ctx, nil)
	return counter, nil
}
