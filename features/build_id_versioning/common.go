package build_id_versioning

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/temporalio/features/harness/go/harness"
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
			ExistingCompatibleBuildID: "2.0",
			MakeSetDefault:            true,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func ServerSupportsBuildIDVersioning(ctx context.Context, r *harness.Runner) (bool, error) {
	// Force to explicitly enable these old tests since they fail in Server 1.25+.
	// This versioning API has been deprecated, and tests need to be rewritten for
	// the new API.
	enable, present := os.LookupEnv("ENABLE_VERSIONING_TESTS")
	if !present || strings.ToLower(enable) != "true" {
		return false, nil
	}

	capabilities, err := r.Client.WorkflowService().GetSystemInfo(ctx, &workflowservice.GetSystemInfoRequest{})
	if err != nil {
		return false, err
	}
	// Also need to make sure dynamic configs are set and no great way to do that besides trying
	_, err = r.Client.GetWorkerBuildIdCompatibility(ctx, &client.GetWorkerBuildIdCompatibilityOptions{
		TaskQueue: r.TaskQueue,
	})
	if err != nil {
		return false, nil
	}
	if capabilities.Capabilities.BuildIdBasedVersioning {
		return true, nil
	}
	return false, nil
}

func MustTimeoutQuery(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	shortTimeout, shortCancel := context.WithTimeout(ctx, 3*time.Second)
	defer shortCancel()
	_, err := r.Client.QueryWorkflow(shortTimeout, run.GetID(), run.GetRunID(), "waiting")
	if err == nil {
		return fmt.Errorf("query should have timed out: %w", err)
	}
	return nil
}
