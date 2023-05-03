package only_appropriate_worker_gets_task

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/features/features/build_id_versioning"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       Execute,
	WorkerOptions: worker.Options{BuildIDForVersioning: "2.1"},
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	if supported, err := build_id_versioning.ServerSupportsBuildIDVersioning(ctx, r.Client); !supported || err != nil {
		if err != nil {
			return nil, err
		}
		return nil, r.Skip(fmt.Sprintf("server does not support build id versioning"))
	}
	// Add some versions to the queue
	err := build_id_versioning.AddSomeVersions(ctx, r.Client, r.TaskQueue)
	if err != nil {
		return nil, err
	}

	// Start workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}
	// Wait a few seconds for query to say it's ready for this worker to stop
	err = r.QueryUntilEventually(ctx, run, "waiting", true, 50*time.Millisecond, 5*time.Second)
	if err != nil {
		return nil, err
	}
	// Stop the worker
	r.StopWorker()
	err = r.ResetStickyQueue(ctx, run)
	// Now issue the signal - if any of the subsequently launched workers is compatible then the
	// workflow will complete.
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "unblocker", nil)
	if err != nil {
		return nil, err
	}

	// Start workers with version `2.0` and `1.0` and make sure they don't get tasks
	for _, version := range []string{"2.0", "1.0"} {
		r.Feature.WorkerOptions.BuildIDForVersioning = version
		err = r.StartWorker()
		if err != nil {
			return nil, err
		}

		// Try a query, which should time out
		err = mustTimeoutQuery(ctx, r, run)
		if err == nil {
			return nil, fmt.Errorf("worker with version %s should not have gotten task", version)
		}

		r.StopWorker()
		err = r.ResetStickyQueue(ctx, run)
		if err != nil {
			return nil, err
		}
	}

	// Complete the workflow with `2.1` worker
	r.Feature.WorkerOptions.BuildIDForVersioning = "2.1"
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	return run, nil
}

func mustTimeoutQuery(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	shortTimeout, shortCancel := context.WithTimeout(ctx, 3*time.Second)
	defer shortCancel()
	_, err := r.Client.QueryWorkflow(shortTimeout, run.GetID(), run.GetRunID(), "waiting")
	// TODO: Invert
	if err != nil {
		return errors.WithMessage(err, "query should have timed out")
	}
	return nil
}

func Workflow(ctx workflow.Context) error {
	// Setup query to tell caller we're waiting
	waiting := false
	err := workflow.SetQueryHandler(ctx, "waiting", func() (bool, error) {
		return waiting, nil
	})
	if err != nil {
		return fmt.Errorf("failed setting query handler: %w", err)
	}

	unblockCh := workflow.GetSignalChannel(ctx, "unblocker")
	waiting = true
	unblockCh.Receive(ctx, nil)

	return nil
}
