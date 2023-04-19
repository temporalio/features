package unversioned_worker_no_task

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/features/features/build_id_versioning"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute:   Execute,
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
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

	shortTimeout, shortCancel := context.WithTimeout(ctx, 3*time.Second)
	defer shortCancel()
	_, err = r.Client.QueryWorkflow(shortTimeout, run.GetID(), run.GetRunID(), "__stack_trace")
	if err == nil {
		return nil, errors.New("query should have timed out")
	}
	r.StopWorker()

	// Restart worker with appropriate version
	r.Feature.WorkerOptions.BuildIDForVersioning = "2.1"
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(_ workflow.Context) error {
	return nil
}
