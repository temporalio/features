package continues_as_new_on_correct_version

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/features/features/build_id_versioning"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       Execute,
	CheckHistory:  CheckHistory,
	WorkerOptions: worker.Options{BuildID: "1.0", UseBuildIDForVersioning: true},
}

var twoWorker worker.Worker

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	if supported, err := build_id_versioning.ServerSupportsBuildIDVersioning(ctx, r.Client); !supported || err != nil {
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
		TaskQueue: r.TaskQueue,
		Operation: &client.BuildIDOpAddNewIDInNewDefaultSet{
			BuildID: "2.0",
		},
	})
	if err != nil {
		return nil, err
	}

	// Verify running on 1.0
	err = r.QueryUntilEventually(ctx, run, "runningOn", "1.0", 50*time.Millisecond, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Unblock the workflow
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "unblocker", ContinueSame)
	if err != nil {
		return nil, err
	}

	// Verify it's still running on 1.0
	latestRun := r.Client.GetWorkflow(ctx, run.GetID(), "")
	err = r.QueryUntilEventually(ctx, latestRun, "runningOn", "1.0", 50*time.Millisecond, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// unblock it again, this time using default
	err = r.Client.SignalWorkflow(ctx, run.GetID(), "", "unblocker", ContinueDefault)
	if err != nil {
		return nil, err
	}

	// Now it should be running on 2.0
	latestRun = r.Client.GetWorkflow(ctx, run.GetID(), "")
	err = r.QueryUntilEventually(ctx, latestRun, "runningOn", "2.0", 50*time.Millisecond, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// unblock to finish
	err = r.Client.SignalWorkflow(ctx, run.GetID(), "", "unblocker", End)

	return run, nil
}

type SignalDecision int

const (
	ContinueSame SignalDecision = iota
	ContinueDefault
	End
)

func Workflow(ctx workflow.Context, myVer string) error {
	err := workflow.SetQueryHandler(ctx, "runningOn", func() (string, error) {
		return myVer, nil
	})
	if err != nil {
		return err
	}

	unblockCh := workflow.GetSignalChannel(ctx, "unblocker")
	var useDefault SignalDecision
	unblockCh.Receive(ctx, &useDefault)

	if useDefault == End {
		return nil
	}

	var canCtx workflow.Context
	if useDefault == ContinueSame {
		canCtx = workflow.WithWorkflowVersioningIntent(ctx, temporal.VersioningIntentCompatible)
	} else {
		canCtx = workflow.WithWorkflowVersioningIntent(ctx, temporal.VersioningIntentDefault)
	}

	return workflow.NewContinueAsNewError(canCtx, Workflow)
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.0 worker
	twoWorker.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
