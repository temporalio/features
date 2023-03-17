package features

import (
	"go.temporal.io/features/features/activity/cancel_try_cancel"
	"go.temporal.io/features/features/activity/retry_on_error"
	"go.temporal.io/features/features/bugs/go/activity_start_race"
	"go.temporal.io/features/features/bugs/go/child_workflow_cancel_panic"
	"go.temporal.io/features/features/child_workflow/result"
	"go.temporal.io/features/features/continue_as_new/continue_as_same"
	"go.temporal.io/features/features/data_converter/binary"
	"go.temporal.io/features/features/data_converter/empty"
	"go.temporal.io/features/features/data_converter/failure"
	"go.temporal.io/features/features/eager_activity/non_remote_activities_worker"
	"go.temporal.io/features/features/query/successful_query"
	"go.temporal.io/features/features/query/timeout_due_to_no_active_workers"
	"go.temporal.io/features/features/query/unexpected_arguments"
	"go.temporal.io/features/features/query/unexpected_query_type_name"
	"go.temporal.io/features/features/query/unexpected_return_type"
	"go.temporal.io/features/features/schedule/backfill"
	"go.temporal.io/features/features/schedule/basic"
	"go.temporal.io/features/features/schedule/cron"
	"go.temporal.io/features/features/schedule/pause"
	"go.temporal.io/features/features/schedule/trigger"
	"go.temporal.io/features/features/signal/external"
	"go.temporal.io/features/features/telemetry/metrics"
	"go.temporal.io/features/features/update/activities"
	"go.temporal.io/features/features/update/intercept"
	"go.temporal.io/features/features/update/non_durable_reject"
	"go.temporal.io/features/features/update/self"
	"go.temporal.io/features/features/update/user_panics"
	"go.temporal.io/features/features/update/validation_replay"
	"go.temporal.io/features/features/update/worker_restart"
	"go.temporal.io/features/harness/go/harness"
)

func init() {
	// Please keep list in alphabetical order by unqualified import package
	// reference/alias
	harness.MustRegisterFeatures(
		activity_start_race.Feature,
		backfill.Feature,
		basic.Feature,
		binary.Feature,
		cancel_try_cancel.Feature,
		child_workflow_cancel_panic.Feature,
		continue_as_same.Feature,
		cron.Feature,
		empty.Feature,
		external.Feature,
		metrics.Feature,
		pause.Feature,
		result.Feature,
		retry_on_error.Feature,
		failure.Feature,
		successful_query.Feature,
		timeout_due_to_no_active_workers.Feature,
		trigger.Feature,
		unexpected_arguments.Feature,
		unexpected_query_type_name.Feature,
		unexpected_return_type.Feature,
		non_remote_activities_worker.Feature,
		non_durable_reject.Feature,
		activities.Feature,
		user_panics.Feature,
		intercept.Feature,
		validation_replay.Feature,
		self.Feature,
		worker_restart.Feature,
	)
}
