package poll_complete_on_shutdown

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/google/uuid"
	"github.com/temporalio/features/harness/go/harness"
)

const (
	workflowCount   = 5
	shutdownTimeout = 5 * time.Second
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: NoopActivity,
	WorkerOptions: worker.Options{
		WorkerStopTimeout: 10 * time.Second,
	},
	Execute:      Execute,
	CheckHistory: func(context.Context, *harness.Runner, client.WorkflowRun) error { return nil },
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	runs := make([]client.WorkflowRun, 0, workflowCount)
	for i := 0; i < workflowCount; i++ {
		run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:                       fmt.Sprintf("%s-%s", r.Feature.Dir, uuid.NewString()),
			TaskQueue:                r.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
			WorkflowTaskTimeout:      5 * time.Second,
		}, Workflow)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	defer func() {
		for _, run := range runs {
			_ = r.Client.TerminateWorkflow(context.Background(), run.GetID(), run.GetRunID(), "feature cleanup")
		}
	}()

	for _, run := range runs {
		if _, err := r.WaitForActivityTaskScheduled(ctx, run, 10*time.Second); err != nil {
			return nil, err
		}
	}

	start := time.Now()
	r.StopWorker()
	if elapsed := time.Since(start); elapsed > shutdownTimeout {
		return nil, fmt.Errorf("worker shutdown took %s, expected <= %s", elapsed, shutdownTimeout)
	}

	workerPollCompleteOnShutdown, err := expectWorkerPollCompleteOnShutdown()
	if err != nil {
		return nil, err
	}
	if workerPollCompleteOnShutdown {
		for _, run := range runs {
			if err := assertNoWorkflowTaskProblems(ctx, r, run); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

func Workflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 10 * time.Second,
		StartToCloseTimeout:    5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
	for {
		if err := workflow.Sleep(ctx, 20*time.Millisecond); err != nil {
			return err
		}
		if err := workflow.ExecuteActivity(ctx, NoopActivity).Get(ctx, nil); err != nil {
			return err
		}
	}
}

func NoopActivity(context.Context) error {
	return nil
}

func assertNoWorkflowTaskProblems(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	iter := r.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return err
		}
		switch event.GetEventType() {
		case enumspb.EVENT_TYPE_WORKFLOW_TASK_FAILED, enumspb.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT:
			return fmt.Errorf("unexpected workflow task problem in %s: %s", run.GetID(), eventSummary(event))
		}
	}
	return nil
}

func expectWorkerPollCompleteOnShutdown() (bool, error) {
	capabilitiesJSON := os.Getenv("FEATURE_NAMESPACE_CAPABILITIES")
	if capabilitiesJSON == "" {
		return false, fmt.Errorf("FEATURE_NAMESPACE_CAPABILITIES is required")
	}
	var capabilities map[string]bool
	if err := json.Unmarshal([]byte(capabilitiesJSON), &capabilities); err != nil {
		return false, fmt.Errorf("invalid FEATURE_NAMESPACE_CAPABILITIES: %w", err)
	}
	value, ok := capabilities["workerPollCompleteOnShutdown"]
	if !ok {
		return false, fmt.Errorf("FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown")
	}
	return value, nil
}

func eventSummary(event *historypb.HistoryEvent) string {
	if failed := event.GetWorkflowTaskFailedEventAttributes(); failed != nil {
		return failed.String()
	}
	if timedOut := event.GetWorkflowTaskTimedOutEventAttributes(); timedOut != nil {
		return timedOut.String()
	}
	return event.String()
}
