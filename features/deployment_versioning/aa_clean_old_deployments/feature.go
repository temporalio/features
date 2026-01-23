package routing_with_ramp

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:  CleanOldDeployments,
	Activities: []any{ListOldDeployments, DeleteDeployment},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		return run, nil
	},
}

func CleanOldDeployments(ctx workflow.Context) (string, error) {
	var deploymentsToClean []string
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	})
	err := workflow.ExecuteActivity(ctx, ListOldDeployments).Get(ctx, &deploymentsToClean)
	if err != nil {
		return "", err
	}

	for _, deployment := range deploymentsToClean {
		err := workflow.ExecuteActivity(ctx, DeleteDeployment, deployment).Get(ctx, nil)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Cleaned %d deployments", len(deploymentsToClean)), nil
}

func ListOldDeployments(ctx context.Context) ([]string, error) {
	tClient := activity.GetClient(ctx)

	allDeployments := make([]string, 0)
	iterator, err := tClient.WorkerDeploymentClient().List(ctx, client.WorkerDeploymentListOptions{})
	if err != nil {
		return nil, err
	}
	for iterator.HasNext() {
		deployment, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		if deployment.CreateTime.Before(time.Now().Add(-time.Hour * 24)) {
			allDeployments = append(allDeployments, deployment.Name)
		}
	}

	if err != nil {
		return nil, err
	}
	return allDeployments, nil
}

func DeleteDeployment(ctx context.Context, deploymentName string) error {
	tClient := activity.GetClient(ctx)
	ns := activity.GetInfo(ctx).WorkflowNamespace

	// Use low-level gRPC API to access routing config
	deploymentInfo, err := tClient.WorkflowService().DescribeWorkerDeployment(
		ctx,
		&workflowservice.DescribeWorkerDeploymentRequest{
			Namespace:      ns,
			DeploymentName: deploymentName,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to describe worker deployment %s: %w", deploymentName, err)
	}

	routingConfig := deploymentInfo.WorkerDeploymentInfo.RoutingConfig
	conflictToken := deploymentInfo.ConflictToken

	// Unset current version if one is set
	if routingConfig != nil && routingConfig.CurrentDeploymentVersion != nil && routingConfig.CurrentDeploymentVersion.BuildId != "" {
		resp, err := tClient.WorkflowService().SetWorkerDeploymentCurrentVersion(ctx, &workflowservice.SetWorkerDeploymentCurrentVersionRequest{
			Namespace:               ns,
			DeploymentName:          deploymentName,
			BuildId:                 "", // Empty to unset
			ConflictToken:           conflictToken,
			Identity:                "feature-deployment-deleter",
			IgnoreMissingTaskQueues: true,
			AllowNoPollers:          true,
		})
		if err != nil {
			return fmt.Errorf("failed to unset current version for deployment %s: %w", deploymentName, err)
		}
		conflictToken = resp.ConflictToken
	}

	// Unset ramping version if one is set
	if routingConfig != nil && routingConfig.RampingDeploymentVersion != nil && routingConfig.RampingDeploymentVersion.BuildId != "" {
		_, err = tClient.WorkflowService().SetWorkerDeploymentRampingVersion(ctx, &workflowservice.SetWorkerDeploymentRampingVersionRequest{
			Namespace:               ns,
			DeploymentName:          deploymentName,
			BuildId:                 "", // Empty to unset
			ConflictToken:           conflictToken,
			Identity:                "feature-deployment-deleter",
			IgnoreMissingTaskQueues: true,
			AllowNoPollers:          true,
		})
		if err != nil {
			return fmt.Errorf("failed to unset ramping version for deployment %s: %w", deploymentName, err)
		}
	}

	// Delete all versions
	for _, version := range deploymentInfo.WorkerDeploymentInfo.VersionSummaries {
		_, err = tClient.WorkflowService().DeleteWorkerDeploymentVersion(ctx,
			&workflowservice.DeleteWorkerDeploymentVersionRequest{
				Namespace:         ns,
				DeploymentVersion: version.DeploymentVersion,
				SkipDrainage:      true,
				Identity:          "feature-deployment-deleter",
			},
		)
		if err != nil {
			return fmt.Errorf("failed to delete deployment version %s for deployment %s: %w",
				version.DeploymentVersion, deploymentName, err)
		}
	}

	// Delete the deployment itself
	tClient.WorkflowService().DeleteWorkerDeployment(ctx,
		&workflowservice.DeleteWorkerDeploymentRequest{
			Namespace:      ns,
			DeploymentName: deploymentName,
			Identity:       "features-deployment-deleter",
		},
	)
	return nil
}
