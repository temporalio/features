package activity_on_same_version

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/features/features/build_id_versioning"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       Execute,
	CheckHistory:  CheckHistory,
	WorkerOptions: worker.Options{BuildIDForVersioning: "1.0"},
}

var twoWorker worker.Worker

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Add some versions to the queue
	err := build_id_versioning.AddSomeVersions(ctx, r.Client, r.TaskQueue)
	if err != nil {
		return nil, err
	}
	r.Worker.RegisterActivityWithOptions(RanBy1Act, activity.RegisterOptions{Name: "RanBy"})

	// Also start a 2.1 activity worker
	twoWorker = worker.New(r.Client, r.RunnerConfig.TaskQueue, worker.Options{
		BuildIDForVersioning:  "2.1",
		DisableWorkflowWorker: true,
	})
	twoWorker.RegisterActivityWithOptions(RanBy2Act, activity.RegisterOptions{Name: "RanBy"})
	err = twoWorker.Start()
	if err != nil {
		return nil, err
	}

	// Start workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(ctx workflow.Context) error {
	actOpts := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	actCtx := workflow.WithActivityOptions(ctx, actOpts)
	// Ensure we can run an activity
	var res int
	err := workflow.ExecuteActivity(ctx, "RanBy").Get(actCtx, &res)
	if err != nil {
		return err
	}
	if res != 1 {
		return errors.New("expected activity to run on version 1.0 worker")
	}

	useDefaultVer := workflow.ActivityOptions{
		// TODO: Use default version
	}
	err = workflow.ExecuteActivity(workflow.WithActivityOptions(actCtx, useDefaultVer), "RanBy").Get(ctx, &res)
	if err != nil {
		return err
	}
	if res != 2 {
		return errors.New("expected activity to run on default version worker")
	}

	return nil
}

func RanBy1Act(_ context.Context) int {
	return 1
}
func RanBy2Act(_ context.Context) int {
	return 2
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.1 worker
	twoWorker.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
