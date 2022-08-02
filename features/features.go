package features

import (
	"go.temporal.io/sdk-features/features/activity/cancel_try_cancel"
	"go.temporal.io/sdk-features/features/activity/retry_on_error"
	"go.temporal.io/sdk-features/features/bugs/go/activity_start_race"
	"go.temporal.io/sdk-features/features/bugs/go/child_workflow_cancel_panic"
	"go.temporal.io/sdk-features/features/query/successful_query"
	"go.temporal.io/sdk-features/features/signal/external"
	"go.temporal.io/sdk-features/features/telemetry/metrics"
	"go.temporal.io/sdk-features/harness/go/harness"
)

func init() {
	harness.MustRegisterFeatures(
		activity_start_race.Feature,
		cancel_try_cancel.Feature,
		child_workflow_cancel_panic.Feature,
		retry_on_error.Feature,
		successful_query.Feature,
		metrics.Feature,
		external.Feature,
	)
}
