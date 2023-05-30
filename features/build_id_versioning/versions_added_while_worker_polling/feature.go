package versions_added_while_worker_polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/features/features/build_id_versioning"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       Execute,
	WorkerOptions: worker.Options{BuildID: "1.0", UseBuildIDForVersioning: true},
}

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

	// Re-jigger the worker so they'll get a task quickly
	r.StopWorker()
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	// Start workflow & process a task
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 1, time.Millisecond*200, time.Second*2)
	if err != nil {
		return nil, err
	}

	// Stop worker & start 1.1 worker
	r.StopWorker()
	err = r.ResetStickyQueue(ctx, run)
	if err != nil {
		return nil, err
	}
	r.Feature.WorkerOptions.BuildID = "1.1"
	r.Feature.WorkerOptions.UseBuildIDForVersioning = true
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	// Send another signal, see that worker gets it *after* we add 1.1 to compat set
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	// Signal should not have been seen yet
	err = build_id_versioning.MustTimeoutQuery(ctx, r, run)
	if err != nil {
		return nil, errors.New("1.1 worker should not have seen task since 1.1 is not yet in sets")
	}

	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: r.TaskQueue,
		Operation: &client.BuildIDOpAddNewCompatibleVersion{
			BuildID:                   "1.1",
			ExistingCompatibleBuildId: "1.0",
		},
	})
	if err != nil {
		return nil, err
	}
	r.StopWorker()
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 2, time.Millisecond*200, time.Second*2)
	if err != nil {
		return nil, err
	}

	// Add 1.2 and see that new tasks aren't received
	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: r.TaskQueue,
		Operation: &client.BuildIDOpAddNewCompatibleVersion{
			BuildID:                   "1.2",
			ExistingCompatibleBuildId: "1.1",
		},
	})
	if err != nil {
		return nil, err
	}
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 2, time.Millisecond*200, time.Second*2)
	if err == nil {
		return nil, errors.New("1.1 worker should not have seen task since 1.2 is now default")
	}
	// Stop and start at 1.2
	r.StopWorker()
	err = r.ResetStickyQueue(ctx, run)
	if err != nil {
		return nil, err
	}
	r.Feature.WorkerOptions.BuildID = "1.2"
	r.Feature.WorkerOptions.UseBuildIDForVersioning = true
	err = r.StartWorker()

	// Cancel workflow to end it
	err = r.Client.CancelWorkflow(ctx, run.GetID(), run.GetRunID())
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(ctx workflow.Context) error {
	counter := 0
	addChan := workflow.GetSignalChannel(ctx, "add1")
	err := workflow.SetQueryHandler(ctx, "counter", func() (int, error) { return counter, nil })
	if err != nil {
		return err
	}

	workflow.Go(ctx,
		func(ctx workflow.Context) {
			for addChan.Receive(ctx, nil) {
				counter += 1
			}
		})

	ctx.Done().Receive(ctx, nil)

	return nil
}
