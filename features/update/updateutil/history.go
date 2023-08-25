package updateutil

import (
	"context"

	"github.com/temporalio/features/harness/go/harness"
	"github.com/temporalio/features/harness/go/history"
	enumspb "go.temporal.io/api/enums/v1"
)

func RequireNoUpdateRejectedEvents(ctx context.Context, runner *harness.Runner) {
	runner.Log.Debug("Checking for verboten workflow update rejected events", "Feature", runner.Feature.Dir)
	fetcher := &history.Fetcher{
		Client:         runner.Client,
		Namespace:      runner.Namespace,
		TaskQueue:      runner.TaskQueue,
		FeatureStarted: runner.CreateTime,
	}
	histories, err := fetcher.Fetch(ctx)
	runner.Require.NoError(err)
	for _, hist := range histories {
		for _, ev := range hist.GetEvents() {
			if ev.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_REJECTED {
				runner.Require.FailNow("found a workflow update rejected event")
			}
		}
	}
	runner.Log.Debug("No histories contained update rejected events", "history-count", len(histories))
}
