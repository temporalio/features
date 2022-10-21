package features

import (
	"go.temporal.io/sdk-features/features/activity/cancel_try_cancel"
	"go.temporal.io/sdk-features/features/activity/retry_on_error"
	"go.temporal.io/sdk-features/features/bugs/go/activity_start_race"
	"go.temporal.io/sdk-features/features/bugs/go/child_workflow_cancel_panic"
	"go.temporal.io/sdk-features/features/child_workflow/result"
	"go.temporal.io/sdk-features/features/continue_as_new/continue_as_same"
	"go.temporal.io/sdk-features/features/data_converter/binary"
	"go.temporal.io/sdk-features/features/data_converter/empty"
	"go.temporal.io/sdk-features/features/query/successful_query"
	"go.temporal.io/sdk-features/features/query/timeout_due_to_no_active_workers"
	"go.temporal.io/sdk-features/features/query/unexpected_arguments"
	"go.temporal.io/sdk-features/features/query/unexpected_query_type_name"
	"go.temporal.io/sdk-features/features/query/unexpected_return_type"
	"go.temporal.io/sdk-features/features/signal/external"
	"go.temporal.io/sdk-features/features/telemetry/metrics"
	"go.temporal.io/sdk-features/harness/go/harness"
)

func init() {
	harness.MustRegisterFeatures(
		activity_start_race.Feature,
		binary.Feature,
		cancel_try_cancel.Feature,
		result.Feature,
		child_workflow_cancel_panic.Feature,
		continue_as_same.Feature,
		retry_on_error.Feature,
		successful_query.Feature,
		timeout_due_to_no_active_workers.Feature,
		unexpected_arguments.Feature,
		unexpected_query_type_name.Feature,
		unexpected_return_type.Feature,
		metrics.Feature,
		empty.Feature,
		external.Feature,
	)
}
