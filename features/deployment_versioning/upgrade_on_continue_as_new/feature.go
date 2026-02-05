package upgrade_on_continue_as_new

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/features/features/deployment_versioning"
	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: []interface{}{
		harness.WorkflowWithOptions{
			Workflow: ContinueAsNewWithVersionUpgrade,
			Options: workflow.RegisterOptions{
				Name:               "ContinueAsNewWithVersionUpgrade",
				VersioningBehavior: workflow.VersioningBehaviorPinned,
			},
		},
	},
	WorkerOptions: worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning: true,
			Version:       v1,
		},
	},
	Execute:         Execute,
	ExpectRunResult: "v2.0",
}

// @@@SNIPSTART upgrade-on-continue-as-new-go
// ContinueAsNewWithVersionUpgrade demonstrates how a pinned Workflow can
// upgrade to a new Worker Deployment Version at Continue-as-New boundaries.
// The Workflow checks for the TARGET_WORKER_DEPLOYMENT_VERSION_CHANGED reason
// and uses AutoUpgrade behavior to move to the new version.
func ContinueAsNewWithVersionUpgrade(ctx workflow.Context, attempt int) (string, error) {
	if attempt > 0 {
		// After continuing as new, return the version
		return "v1.0", nil
	}

	// Check continue-as-new-suggested periodically
	for {
		err := workflow.Sleep(ctx, 10*time.Millisecond)
		if err != nil {
			return "", err
		}

		info := workflow.GetInfo(ctx)
		if info.GetContinueAsNewSuggested() {
			for _, reason := range info.GetContinueAsNewSuggestedReasons() {
				if reason == workflow.ContinueAsNewSuggestedReasonTargetWorkerDeploymentVersionChanged {
					// A new Worker Deployment Version is available
					// Continue-as-New with upgrade to the new version
					return "", workflow.NewContinueAsNewErrorWithOptions(
						ctx,
						workflow.ContinueAsNewErrorOptions{
							InitialVersioningBehavior: workflow.ContinueAsNewVersioningBehaviorAutoUpgrade,
						},
						"ContinueAsNewWithVersionUpgrade",
						attempt+1,
					)
				}
			}
		}
	}
}

// @@@SNIPEND

func ContinueAsNewWithVersionUpgradeV2(
	ctx workflow.Context,
	attempt int,
) (string, error) {
	return "v2.0", nil
}

var deploymentName = uuid.NewString()
var v1 = worker.WorkerDeploymentVersion{
	DeploymentName: deploymentName,
	BuildID:        "1.0",
}
var v2 = worker.WorkerDeploymentVersion{
	DeploymentName: deploymentName,
	BuildID:        "2.0",
}

var worker2 worker.Worker

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	if supported := deployment_versioning.ServerSupportsDeployments(ctx, r); !supported {
		return nil, r.Skip(fmt.Sprintf("server does not support deployment versioning"))
	}

	// Two workers:
	// 1.0) and 2.0) both with no default versioning behavior
	// SetCurrent to 1.0
	// Workflow (annotated as Pinned):
	// - Start and wait for continue-as-new-suggested boolean
	// - If continue-as-new-suggested is true and the reason is version-changed, continue as new with AutoUpgrade behavior
	// Verify workflow returns 2.0.

	// Define v2.0 of the workflow and start polling
	worker2 = worker.New(r.Client, r.TaskQueue, worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning: true,
			Version:       v2,
		},
	})
	worker2.RegisterWorkflowWithOptions(ContinueAsNewWithVersionUpgradeV2, workflow.RegisterOptions{
		Name:               "ContinueAsNewWithVersionUpgrade",
		VersioningBehavior: workflow.VersioningBehaviorPinned,
	})
	if err := worker2.Start(); err != nil {
		return nil, err
	}

	// Wait for the deployment to be ready
	dHandle := r.Client.WorkerDeploymentClient().GetHandle(deploymentName)
	if err := deployment_versioning.WaitForDeployment(r, ctx, dHandle); err != nil {
		return nil, err
	}

	// Wait for version 1.0 to be ready
	if err := deployment_versioning.WaitForDeploymentVersion(r, ctx, dHandle, v1); err != nil {
		return nil, err
	}
	// Set version 1.0 as current
	if err := deployment_versioning.SetCurrent(r, ctx, deploymentName, v1); err != nil {
		return nil, err
	}

	// Wait for v1.0-as-Current Deployment Routing Config to be propagated to all task queues
	if err := deployment_versioning.WaitForWorkerDeploymentRoutingConfigPropagation(r, ctx, deploymentName, v1.BuildID, ""); err != nil {
		return nil, err
	}

	run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue:                r.TaskQueue,
		ID:                       "continueasnew-with-version-upgrade",
		WorkflowExecutionTimeout: 1 * time.Minute,
	}, "ContinueAsNewWithVersionUpgrade", 0)
	if err != nil {
		return nil, err
	}

	// Wait for workflow to complete one WFT on v1.0
	if err := deployment_versioning.WaitForWorkflowRunning(r, ctx, run); err != nil {
		return nil, err
	}
	// Wait for version 2.0 to be ready
	if err := deployment_versioning.WaitForDeploymentVersion(r, ctx, dHandle, v2); err != nil {
		return nil, err
	}
	// Set version 2.0 as current
	if err := deployment_versioning.SetCurrent(r, ctx, deploymentName, v2); err != nil {
		return nil, err
	}
	// Wait for v2.0-as-Current Deployment Routing Config to be propagated to all task queues
	if err := deployment_versioning.WaitForWorkerDeploymentRoutingConfigPropagation(r, ctx, deploymentName, v2.BuildID, ""); err != nil {
		return nil, err
	}
	return run, nil
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.0 worker
	worker2.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
