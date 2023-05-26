package build_id_versioning

import (
	"context"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// AddSomeVersions adds {1.0} {2.0, 2.1} to the task queue with 2.1 as default
func AddSomeVersions(ctx context.Context, c client.Client, tq string) error {
	for _, version := range []string{"1.0", "2.0"} {
		err := c.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
			TaskQueue: tq,
			Operation: &client.BuildIDOpAddNewIDInNewDefaultSet{
				BuildID: version,
			},
		})
		if err != nil {
			return err
		}
	}

	err := c.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue: tq,
		Operation: &client.BuildIDOpAddNewCompatibleVersion{
			BuildID:                   "2.1",
			ExistingCompatibleBuildId: "2.0",
			MakeSetDefault:            true,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func ServerSupportsBuildIDVersioning(ctx context.Context, c client.Client) (bool, error) {
	capabilities, err := c.WorkflowService().GetSystemInfo(ctx, &workflowservice.GetSystemInfoRequest{})
	if err != nil {
		return false, err
	}
	if capabilities.Capabilities.BuildIdBasedVersioning {
		return true, nil
	}
	return false, nil
}
