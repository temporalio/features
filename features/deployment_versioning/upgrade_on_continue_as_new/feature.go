package upgrade_on_continue_as_new

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

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

		if info := workflow.GetInfo(ctx); info.GetContinueAsNewSuggested() {
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
