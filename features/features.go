package features

import (
	"go.temporal.io/sdk-features/features/activity/retry_on_error"
	"go.temporal.io/sdk-features/features/bugs/go/child_workflow_cancel_panic"
	"go.temporal.io/sdk-features/features/telemetry/metrics"
	"go.temporal.io/sdk-features/harness/go/harness"
)

func init() {
	harness.MustRegisterFeatures(
		child_workflow_cancel_panic.Feature,
		retry_on_error.Feature,
		metrics.Feature,
	)
}
