package activity_on_same_version

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/temporalio/features/features/build_id_versioning"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: []interface{}{Workflow, harness.WorkflowWithOptions{Workflow: RanBy1Child, Options: workflow.RegisterOptions{Name: "RanByWf"}}},
	Activities: harness.ActivityWithOptions{
		Activity: RanBy1Act,
		Options:  activity.RegisterOptions{Name: "RanBy"},
	},
	Execute:       Execute,
	CheckHistory:  CheckHistory,
	WorkerOptions: worker.Options{BuildID: "1.0", UseBuildIDForVersioning: true},
}

var twoWorker worker.Worker

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	if supported, err := build_id_versioning.ServerSupportsBuildIDVersioning(ctx, r); !supported || err != nil {
		if err != nil {
			return nil, err
		}
		return nil, r.Skip(fmt.Sprintf("server does not support build id versioning"))
	}
	// Add 1.0 to the queue
	err := r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: r.TaskQueue,
		Operation: &client.BuildIDOpAddNewIDInNewDefaultSet{
			BuildID: "1.0",
		},
	})
	if err != nil {
		return nil, err
	}

	// Also start a 2.0 activity worker
	twoWorker = worker.New(r.Client, r.RunnerConfig.TaskQueue, worker.Options{
		BuildID:                 "2.0",
		UseBuildIDForVersioning: true,
	})
	twoWorker.RegisterActivityWithOptions(RanBy2Act, activity.RegisterOptions{Name: "RanBy"})
	twoWorker.RegisterWorkflowWithOptions(RanBy2Child, workflow.RegisterOptions{Name: "RanByWf"})
	twoWorker.RegisterWorkflow(Workflow)
	err = twoWorker.Start()
	if err != nil {
		return nil, err
	}

	// Start workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Give time for first task to get queued
	time.Sleep(1 * time.Second)

	// Add 2.0 to the queue
	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: r.TaskQueue,
		Operation: &client.BuildIDOpAddNewIDInNewDefaultSet{
			BuildID: "2.0",
		},
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
		VersioningIntent:       temporal.VersioningIntentDefault,
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	err = workflow.ExecuteActivity(workflow.WithActivityOptions(actCtx, useDefaultVer), "RanBy").Get(ctx, &res)
	if err != nil {
		return err
	}
	if res != 2 {
		return errors.New("expected activity to run on default version worker")
	}
	useDefaultVerChild := workflow.ChildWorkflowOptions{
		VersioningIntent: temporal.VersioningIntentDefault,
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

func RanBy1Act(_ context.Context) (int, error) {
	return 1, nil
}
func RanBy1Child(_ workflow.Context) (int, error) {
	return 1, nil
}

func RanBy2Act(_ context.Context) (int, error) {
	return 2, nil
}
func RanBy2Child(_ workflow.Context) (int, error) {
	return 2, nil
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.0 worker
	twoWorker.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
