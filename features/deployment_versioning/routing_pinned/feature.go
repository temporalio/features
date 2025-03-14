package routing_pinned

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/features/deployment_versioning"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var deploymentName = uuid.NewString()

// Wrap for correct Feature.Path
func WaitForSignalOne(ctx workflow.Context) (string, error) {
	return deployment_versioning.WaitForSignalOne(ctx)
}

var Feature = harness.Feature{
	Workflows: []interface{}{
		harness.WorkflowWithOptions{
			Workflow: WaitForSignalOne,
			Options: workflow.RegisterOptions{
				Name:               "WaitForSignal",
				VersioningBehavior: workflow.VersioningBehaviorPinned,
			},
		},
	},
	Execute: Execute,
	WorkerOptions: worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning: true,
			Version:       deploymentName + ".1.0",
		},
	},
	CheckHistory:    CheckHistory,
	ExpectRunResult: "prefix_v1",
}
var worker2 worker.Worker

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	worker2 = deployment_versioning.StartWorker(ctx, r, deploymentName+".2.0",
		workflow.VersioningBehaviorAutoUpgrade)
	if err := worker2.Start(); err != nil {
		return nil, err
	}

	if err := deployment_versioning.SetCurrent(r, ctx, deploymentName, deploymentName+".1.0"); err != nil {
		return nil, err
	}

	run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue:                r.TaskQueue,
		ID:                       "workflow_1",
		WorkflowExecutionTimeout: 1 * time.Minute,
	}, "WaitForSignal")

	if err != nil {
		return nil, err
	}

	if err := deployment_versioning.WaitForWorkflowRunning(r, ctx, run); err != nil {
		return nil, err
	}

	if err := deployment_versioning.SetCurrent(r, ctx, deploymentName, deploymentName+".2.0"); err != nil {
		return nil, err
	}

	if err := deployment_versioning.SignalAll(r, ctx, []client.WorkflowRun{run}); err != nil {
		return nil, err
	}

	return run, nil
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.0 worker
	worker2.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
