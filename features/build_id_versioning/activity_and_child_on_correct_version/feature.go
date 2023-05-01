package activity_on_same_version

import (
	"context"
	"time"

	"github.com/pkg/errors"
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
	// Add 1.0 to the queue
	err := r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:     r.TaskQueue,
		WorkerBuildID: "1.0",
		BecomeDefault: true,
	})
	if err != nil {
		return nil, err
	}
	r.Worker.RegisterActivityWithOptions(RanBy1Act, activity.RegisterOptions{Name: "RanBy"})
	r.Worker.RegisterWorkflowWithOptions(RanBy1Child, workflow.RegisterOptions{Name: "RanByWf"})

	// Also start a 2.0 activity worker
	twoWorker = worker.New(r.Client, r.RunnerConfig.TaskQueue, worker.Options{
		BuildIDForVersioning: "2.0",
	})
	twoWorker.RegisterActivityWithOptions(RanBy2Act, activity.RegisterOptions{Name: "RanBy"})
	twoWorker.RegisterWorkflowWithOptions(RanBy2Child, workflow.RegisterOptions{Name: "RanByWf"})
	err = twoWorker.Start()
	if err != nil {
		return nil, err
	}

	// Start workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Add 2.0 to the queue
	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:     r.TaskQueue,
		WorkerBuildID: "2.0",
		BecomeDefault: true,
	})
	if err != nil {
		return nil, err
	}

	// Unblock the workflow
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "unblocker", nil)
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(ctx workflow.Context) error {
	unblockCh := workflow.GetSignalChannel(ctx, "unblocker")
	unblockCh.Receive(ctx, nil)

	actOpts := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	actCtx := workflow.WithActivityOptions(ctx, actOpts)
	childOpts := workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 5 * time.Second,
	}
	childCtx := workflow.WithChildOptions(ctx, childOpts)

	var res int
	err := workflow.ExecuteActivity(actCtx, "RanBy").Get(ctx, &res)
	if err != nil {
		return err
	}
	if res != 1 {
		return errors.New("expected activity to run on version 1.0 worker")
	}
	err = workflow.ExecuteChildWorkflow(childCtx, "RanByWf").Get(ctx, &res)
	if err != nil {
		return err
	}
	if res != 1 {
		return errors.New("expected child wf to run on version 1.0 worker")
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
	useDefaultVerChild := workflow.ChildWorkflowOptions{
		// TODO: Use default version
	}
	err = workflow.ExecuteChildWorkflow(workflow.WithChildOptions(childCtx, useDefaultVerChild), "RanByWf").Get(ctx, &res)
	if err != nil {
		return err
	}
	if res != 2 {
		return errors.New("expected child wf to run on version 1.0 worker")
	}

	return nil
}

func RanBy1Act(_ context.Context) int {
	return 1
}
func RanBy1Child(_ workflow.Context) int {
	return 1
}

func RanBy2Act(_ context.Context) int {
	return 2
}
func RanBy2Child(_ workflow.Context) int {
	return 2
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.1 worker
	twoWorker.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
