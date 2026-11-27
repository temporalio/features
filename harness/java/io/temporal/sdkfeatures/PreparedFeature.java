package io.temporal.sdkfeatures;

import io.temporal.common.metadata.POJOWorkflowImplMetadata;

public class PreparedFeature {

  static PreparedFeature[] ALL =
      PreparedFeature.prepareFeatures(
          activity.basic_no_workflow_timeout.feature.Impl.class,
          activity.retry_on_error.feature.Impl.class,
          activity.cancel_try_cancel.feature.Impl.class,
          activity.shutdown.feature.Impl.class,
          activity.standalone.feature.Impl.class,
          child_workflow.result.feature.Impl.class,
          child_workflow.signal.feature.Impl.class,
          client.http_proxy.feature.Impl.class,
          client.http_proxy_auth.feature.Impl.class,
          continue_as_new.continue_as_same.feature.Impl.class,
          data_converter.binary.feature.Impl.class,
          data_converter.binary_protobuf.feature.Impl.class,
          data_converter.codec.feature.Impl.class,
          data_converter.empty.feature.Impl.class,
          data_converter.json.feature.Impl.class,
          data_converter.json_protobuf.feature.Impl.class,
          eager_activity.non_remote_activities_worker.feature.Impl.class,
          external_storage.s3_interop.feature.Impl.class,
          nexus.async_cancellation.feature.Impl.class,
          nexus.async_cancellation_abandon.feature.Impl.class,
          nexus.async_cancellation_try_cancel.feature.Impl.class,
          nexus.async_cancellation_wait_requested.feature.Impl.class,
          nexus.async_operation_error.feature.Impl.class,
          nexus.async_schedule_to_close_timeout.feature.Impl.class,
          nexus.async_start_to_close_timeout.feature.Impl.class,
          nexus.async_success.feature.Impl.class,
          nexus.operation_not_found.feature.Impl.class,
          nexus.parallel_sync_operations.feature.Impl.class,
          nexus.schedule_to_close_timeout.feature.Impl.class,
          nexus.schedule_to_start_timeout.feature.Impl.class,
          nexus.service_not_found.feature.Impl.class,
          nexus.standalone.async_failure.feature.Impl.class,
          nexus.standalone.async_success.feature.Impl.class,
          nexus.standalone.sync_failure.feature.Impl.class,
          nexus.standalone.sync_handler_error_non_retryable.feature.Impl.class,
          nexus.standalone.sync_success.feature.Impl.class,
          nexus.sync_handler_error_non_retryable.feature.Impl.class,
          nexus.sync_operation_canceled.feature.Impl.class,
          nexus.sync_operation_error.feature.Impl.class,
          nexus.sync_operation_error_codec.feature.Impl.class,
          nexus.sync_success.feature.Impl.class,
          nexus.sync_success_codec.feature.Impl.class,
          nexus.sync_success_codec_retry.feature.Impl.class,
          nexus.sync_unexpected_handler_error.feature.Impl.class,
          query.successful_query.feature.Impl.class,
          query.timeout_due_to_no_active_workers.feature.Impl.class,
          query.unexpected_arguments.feature.Impl.class,
          query.unexpected_query_type_name.feature.Impl.class,
          query.unexpected_return_type.feature.Impl.class,
          schedule.backfill.feature.Impl.class,
          schedule.basic.feature.Impl.class,
          schedule.cron.feature.Impl.class,
          schedule.duplicate_error.feature.Impl.class,
          schedule.pause.feature.Impl.class,
          schedule.trigger.feature.Impl.class,
          serialization_context.activity_payloads.feature.Impl.class,
          serialization_context.async_activity_completion.feature.Impl.class,
          serialization_context.child_workflow_payloads.feature.Impl.class,
          serialization_context.child_workflow_payloads_default_id.feature.Impl.class,
          serialization_context.continue_as_new.feature.Impl.class,
          serialization_context.external_signal.feature.Impl.class,
          serialization_context.failure.feature.Impl.class,
          serialization_context.local_activity_payloads.feature.Impl.class,
          serialization_context.workflow_payloads.feature.Impl.class,
          signal.external.feature.Impl.class,
          update.activities.feature.Impl.class,
          update.async_accepted.feature.Impl.class,
          update.deduplication.feature.Impl.class,
          update.client_interceptor.feature.Impl.class,
          update.non_durable_reject.feature.Impl.class,
          update.task_failure.feature.Impl.class,
          update.worker_restart.feature.Impl.class,
          update.validation_replay.feature.Impl.class,
          update.self.feature.Impl.class,
          worker_shutdown.poll_complete_on_shutdown.feature.Impl.class);

  @SafeVarargs
  static PreparedFeature[] prepareFeatures(Class<? extends Feature>... classes) {
    var ret = new PreparedFeature[classes.length];
    for (int i = 0; i < classes.length; i++) {
      ret[i] = new PreparedFeature(classes[i]);
    }
    return ret;
  }

  final Class<? extends Feature> factoryClass;
  public final POJOWorkflowImplMetadata metadata;
  public final String dir;

  PreparedFeature(Class<? extends Feature> factoryClass) {
    this.factoryClass = factoryClass;
    this.metadata = POJOWorkflowImplMetadata.newInstance(factoryClass);
    // Directory is the package, but slashes instead of dots. We use string
    // instead of nio Path because we don't want platform-specific separator.
    dir = factoryClass.getPackageName().replace('.', '/');
  }

  Feature newInstance() {
    try {
      return factoryClass.getDeclaredConstructor().newInstance();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }
}
