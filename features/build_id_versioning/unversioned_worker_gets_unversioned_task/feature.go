package unversioned_worker_gets_unversioned_task

import (
	"context"
	"fmt"

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
	if supported, err := build_id_versioning.ServerSupportsBuildIDVersioning(ctx, r); !supported || err != nil {
		if err != nil {
			return nil, err
		}
		return nil, r.Skip(fmt.Sprintf("server does not support build id versioning"))
	}

	// Start a workflow that won't be versioned
	unVersionedRun, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Add some versions to the queue
	err = build_id_versioning.AddSomeVersions(ctx, r.Client, r.TaskQueue)
	if err != nil {
		return nil, err
	}

	// Now unblock the unversioned WF, it should complete
	err = r.Client.SignalWorkflow(ctx, unVersionedRun.GetID(), unVersionedRun.GetRunID(), "unblocker", nil)
	if err != nil {
		return nil, err
	}

	return unVersionedRun, nil
}

func Workflow(ctx workflow.Context) error {
	unblockCh := workflow.GetSignalChannel(ctx, "unblocker")
	unblockCh.Receive(ctx, nil)
	return nil
}
