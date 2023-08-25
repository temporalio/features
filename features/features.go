package features

import (
	"github.com/temporalio/features/features/activity/cancel_try_cancel"
	"github.com/temporalio/features/features/activity/retry_on_error"
	"github.com/temporalio/features/features/bugs/go/activity_start_race"
	"github.com/temporalio/features/features/bugs/go/child_workflow_cancel_panic"
	activity_on_same_version "github.com/temporalio/features/features/build_id_versioning/activity_and_child_on_correct_version"
	"github.com/temporalio/features/features/build_id_versioning/continues_as_new_on_correct_version"
	"github.com/temporalio/features/features/build_id_versioning/only_appropriate_worker_gets_task"
	"github.com/temporalio/features/features/build_id_versioning/unversioned_worker_gets_unversioned_task"
	"github.com/temporalio/features/features/build_id_versioning/unversioned_worker_no_task"
	"github.com/temporalio/features/features/build_id_versioning/versions_added_while_worker_polling"
	"github.com/temporalio/features/features/child_workflow/result"
	"github.com/temporalio/features/features/child_workflow/signal"
	"github.com/temporalio/features/features/continue_as_new/continue_as_same"
	"github.com/temporalio/features/features/data_converter/binary"
	"github.com/temporalio/features/features/data_converter/binary_protobuf"
	"github.com/temporalio/features/features/data_converter/codec"
	"github.com/temporalio/features/features/data_converter/empty"
	"github.com/temporalio/features/features/data_converter/failure"
	"github.com/temporalio/features/features/data_converter/json"
	"github.com/temporalio/features/features/data_converter/json_protobuf"
	"github.com/temporalio/features/features/eager_activity/non_remote_activities_worker"
	"github.com/temporalio/features/features/query/successful_query"
	"github.com/temporalio/features/features/query/timeout_due_to_no_active_workers"
	"github.com/temporalio/features/features/query/unexpected_arguments"
	"github.com/temporalio/features/features/query/unexpected_query_type_name"
	"github.com/temporalio/features/features/query/unexpected_return_type"
	"github.com/temporalio/features/features/reset/reset_and_delete"
	"github.com/temporalio/features/features/schedule/backfill"
	"github.com/temporalio/features/features/schedule/basic"
	"github.com/temporalio/features/features/schedule/cron"
	"github.com/temporalio/features/features/schedule/pause"
	"github.com/temporalio/features/features/schedule/trigger"
	"github.com/temporalio/features/features/signal/external"
	"github.com/temporalio/features/features/telemetry/metrics"
	update_async_accepted "github.com/temporalio/features/features/update/activities"
	update_activities "github.com/temporalio/features/features/update/async_accepted"
	update_deduplication "github.com/temporalio/features/features/update/deduplication"
	update_intercept "github.com/temporalio/features/features/update/intercept"
	update_non_durable_reject "github.com/temporalio/features/features/update/non_durable_reject"
	update_self "github.com/temporalio/features/features/update/self"
	update_task_failure "github.com/temporalio/features/features/update/task_failure"
	update_validation_replay "github.com/temporalio/features/features/update/validation_replay"
	update_worker_restart "github.com/temporalio/features/features/update/worker_restart"
	"github.com/temporalio/features/harness/go/harness"
)

func init() {
	// Please keep list in alphabetical order by unqualified import package
	// reference/alias
	harness.MustRegisterFeatures(
		activity_start_race.Feature,
		backfill.Feature,
		basic.Feature,
		binary.Feature,
		binary_protobuf.Feature,
		cancel_try_cancel.Feature,
		child_workflow_cancel_panic.Feature,
		continue_as_same.Feature,
		codec.Feature,
		cron.Feature,
		empty.Feature,
		external.Feature,
		json.Feature,
		json_protobuf.Feature,
		metrics.Feature,
		pause.Feature,
		result.Feature,
		retry_on_error.Feature,
		failure.Feature,
		signal.Feature,
		successful_query.Feature,
		timeout_due_to_no_active_workers.Feature,
		trigger.Feature,
		unexpected_arguments.Feature,
		unexpected_query_type_name.Feature,
		unexpected_return_type.Feature,
		non_remote_activities_worker.Feature,
		update_activities.Feature,
		update_async_accepted.Feature,
		update_deduplication.Feature,
		update_intercept.Feature,
		update_non_durable_reject.Feature,
		update_self.Feature,
		update_task_failure.Feature,
		update_validation_replay.Feature,
		update_worker_restart.Feature,
		only_appropriate_worker_gets_task.Feature,
		unversioned_worker_no_task.Feature,
		versions_added_while_worker_polling.Feature,
		activity_on_same_version.Feature,
		continues_as_new_on_correct_version.Feature,
		unversioned_worker_gets_unversioned_task.Feature,
		reset_and_delete.Feature,
	)
}
