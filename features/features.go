package features

import (
	"go.temporal.io/features/features/activity/cancel_try_cancel"
	"go.temporal.io/features/features/activity/retry_on_error"
	"go.temporal.io/features/features/bugs/go/activity_start_race"
	"go.temporal.io/features/features/bugs/go/child_workflow_cancel_panic"
	"go.temporal.io/features/features/build_id_versioning/only_appropriate_worker_gets_task"
	"go.temporal.io/features/features/build_id_versioning/unversioned_worker_no_task"
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
	update_async_accepted "go.temporal.io/features/features/update/activities"
	update_activities "go.temporal.io/features/features/update/async_accepted"
	update_intercept "go.temporal.io/features/features/update/intercept"
	update_non_durable_reject "go.temporal.io/features/features/update/non_durable_reject"
	update_self "go.temporal.io/features/features/update/self"
	update_task_failure "go.temporal.io/features/features/update/task_failure"
	update_validation_replay "go.temporal.io/features/features/update/validation_replay"
	update_worker_restart "go.temporal.io/features/features/update/worker_restart"
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
		update_activities.Feature,
		update_async_accepted.Feature,
		update_intercept.Feature,
		update_non_durable_reject.Feature,
		update_self.Feature,
		update_task_failure.Feature,
		update_validation_replay.Feature,
		update_worker_restart.Feature,
		only_appropriate_worker_gets_task.Feature,
		unversioned_worker_no_task.Feature,
	)
}
