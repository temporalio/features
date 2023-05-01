package build_id_versioning

import (
	"context"

	"go.temporal.io/sdk/client"
)

// AddSomeVersions adds {1.0} {2.0, 2.1} to the task queue with 2.1 as default
func AddSomeVersions(ctx context.Context, c client.Client, tq string) error {
	for _, version := range []string{"1.0", "2.0"} {
		err := c.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
			TaskQueue:     tq,
			WorkerBuildID: version,
		})
		if err != nil {
			return err
		}
	}

	err := c.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:         tq,
		WorkerBuildID:     "2.1",
		CompatibleBuildID: "2.0",
		BecomeDefault:     true,
	})
	if err != nil {
		return err
	}

	return nil
}
