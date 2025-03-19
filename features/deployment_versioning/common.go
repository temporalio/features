package deployment_versioning

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func StartWorker(ctx context.Context, r *harness.Runner, version string, versioningBehavior workflow.VersioningBehavior, waitForSignal func(workflow.Context) (string, error)) worker.Worker {
	w := worker.New(r.Client, r.TaskQueue, worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning:             true,
			Version:                   version,
			DefaultVersioningBehavior: versioningBehavior,
		},
	})
	w.RegisterWorkflowWithOptions(waitForSignal, workflow.RegisterOptions{
		Name: "WaitForSignal",
	})
	return w
}

func WaitForDeploymentVersion(r *harness.Runner, ctx context.Context, dHandle client.WorkerDeploymentHandle, version string) error {
	return r.DoUntilEventually(ctx, 300*time.Millisecond, 10*time.Second,
		func() bool {
			d, err := dHandle.Describe(ctx, client.WorkerDeploymentDescribeOptions{})
			if err != nil {
				return false
			}
			for _, v := range d.Info.VersionSummaries {
				if v.Version == version {
					return true
				}
			}
			return false
		})
}

func WaitForDeployment(r *harness.Runner, ctx context.Context, dHandle client.WorkerDeploymentHandle) error {
	return r.DoUntilEventually(ctx, 300*time.Millisecond, 10*time.Second,
		func() bool {
			_, err := dHandle.Describe(ctx, client.WorkerDeploymentDescribeOptions{})
			return err == nil
		})
}

func WaitForWorkflowRunning(r *harness.Runner, ctx context.Context, handle client.WorkflowRun) error {
	return r.DoUntilEventually(ctx, 300*time.Millisecond, 10*time.Second,
		func() bool {
			describeResp, err := r.Client.DescribeWorkflowExecution(ctx, handle.GetID(), handle.GetRunID())
			if err != nil {
				return false
			}
			status := describeResp.WorkflowExecutionInfo.Status
			return enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING == status
		})
}

func SetCurrent(r *harness.Runner, ctx context.Context, deploymentName string, version string) error {
	dHandle := r.Client.WorkerDeploymentClient().GetHandle(deploymentName)

	if err := WaitForDeployment(r, ctx, dHandle); err != nil {
		return err
	}

	response1, err := dHandle.Describe(ctx, client.WorkerDeploymentDescribeOptions{})
	if err != nil {
		return err
	}

	if err := WaitForDeploymentVersion(r, ctx, dHandle, version); err != nil {
		return err
	}

	_, err = dHandle.SetCurrentVersion(ctx, client.WorkerDeploymentSetCurrentVersionOptions{
		Version:       version,
		ConflictToken: response1.ConflictToken,
	})

	return err
}

func SetRamp(r *harness.Runner, ctx context.Context, deploymentName string, version string, percentage float32) error {
	dHandle := r.Client.WorkerDeploymentClient().GetHandle(deploymentName)

	if err := WaitForDeployment(r, ctx, dHandle); err != nil {
		return err
	}

	response1, err := dHandle.Describe(ctx, client.WorkerDeploymentDescribeOptions{})
	if err != nil {
		return err
	}

	if err := WaitForDeploymentVersion(r, ctx, dHandle, version); err != nil {
		return err
	}

	_, err = dHandle.SetRampingVersion(ctx, client.WorkerDeploymentSetRampingVersionOptions{
		Version:       version,
		ConflictToken: response1.ConflictToken,
		Percentage:    float32(100.0),
	})

	return err
}

func ServerSupportsDeployments(ctx context.Context, r *harness.Runner) bool {
	// No system capability, only dynamic config in namespace, need to just try...
	iter, err := r.Client.WorkerDeploymentClient().List(ctx, client.WorkerDeploymentListOptions{})
	if err != nil {
		return false
	}
	// Need to call `HasNext` to contact the server
	for iter.HasNext() {
		_, err := iter.Next()
		if err != nil {
			return false
		}
	}
	return true
}
