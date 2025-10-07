package shutdown

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  Workflow,
	Activities: activities,
	WorkerOptions: worker.Options{
		WorkerStopTimeout: 1 * time.Second,
	},
	Execute:         Execute,
	ExpectRunResult: "done",
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Wait for activity task to be scheduled
	_, err = r.WaitForActivityTaskScheduled(ctx, run, 5*time.Second)
	if err != nil {
		return nil, err
	}

	r.StopWorker()
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(ctx workflow.Context) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 300 * time.Millisecond,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})

	fut := workflow.ExecuteActivity(ctx, activities.CancelSuccess)
	fut1 := workflow.ExecuteActivity(ctx, activities.CancelFailure)
	fut2 := workflow.ExecuteActivity(ctx, activities.CancelIgnore)

	err := fut.Get(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("expected activity to succeed, got %v", err)
	}

	err = fut1.Get(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "worker is shutting down") {
		return "", fmt.Errorf("expected activity to fail with 'worker is shutting down', got %v", err)
	}

	err = fut2.Get(ctx, nil)
	if !strings.Contains(err.Error(), "(type: ScheduleToClose)") {
		return "", fmt.Errorf("expected activity to fail with ScheduleToClose timeout, got %v", err)
	}

	return "done", nil
}

var activities = &Activities{}

type Activities struct{}

func (a *Activities) CancelSuccess(ctx context.Context) error {
	<-activity.GetWorkerStopChannel(ctx)
	return nil
}

func (a *Activities) CancelFailure(ctx context.Context) error {
	<-activity.GetWorkerStopChannel(ctx)
	return fmt.Errorf("worker is shutting down")
}

func (a *Activities) CancelIgnore(ctx context.Context) error {
	time.Sleep(15 * time.Second)
	return nil
}
