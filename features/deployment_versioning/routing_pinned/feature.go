package routing_pinned

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

var deploymentName = uuid.NewString()

func WaitForSignalOne(ctx workflow.Context) (string, error) {
	var value string
	workflow.GetSignalChannel(ctx, "start-signal").Receive(ctx, &value)
	return value + "_v1", nil
}

func WaitForSignalTwo(ctx workflow.Context) (string, error) {
	var value string
	workflow.GetSignalChannel(ctx, "start-signal").Receive(ctx, &value)
	return value + "_v2", nil
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
	if supported := deployment_versioning.ServerSupportsDeployments(ctx, r); !supported {
		return nil, r.Skip(fmt.Sprintf("server does not support deployment versioning"))
	}

	worker2 = deployment_versioning.StartWorker(ctx, r, deploymentName+".2.0",
		workflow.VersioningBehaviorAutoUpgrade, WaitForSignalTwo)
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

	if err := r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "start-signal", "prefix"); err != nil {
		return nil, err
	}

	return run, nil
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Shut down the 2.0 worker
	worker2.Stop()
	return r.CheckHistoryDefault(ctx, run)
}
