package io.temporal.sdkfeatures;

import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.WorkflowOptions;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.worker.WorkerOptions;
import io.temporal.workflow.Workflow;
import java.util.function.Consumer;

public interface Feature {

  default <T> T activities(Class<T> activityIface, Consumer<ActivityOptions.Builder> builderFunc) {
    var builder = ActivityOptions.newBuilder();
    builderFunc.accept(builder);
    return Workflow.newActivityStub(activityIface, builder.build());
  }

  default void workflowServiceOptions(WorkflowServiceStubsOptions.Builder builder) {}

  default void workflowClientOptions(WorkflowClientOptions.Builder builder) {}

  default void workerFactoryOptions(WorkerFactoryOptions.Builder builder) {}

  default void workerOptions(WorkerOptions.Builder builder) {}

  default void workflowOptions(WorkflowOptions.Builder builder) {}

  default boolean workerUsesProxy() {
    return false;
  }

  default boolean initiatorUsesProxy() {
    return true;
  }

  default Run execute(Runner runner) throws Exception {
    return runner.executeSingleParameterlessWorkflow();
  }

  default void checkResult(Runner runner, Run run) throws Exception {
    // Just wait for result so it can throw if there's an error
    runner.waitForRunResult(run);
  }

  default void checkHistory(Runner runner, Run run) throws Exception {
    runner.checkCurrentAndPastHistories(run);
  }

  // This may be used to e.g. register additional workflow classes
  default void prepareWorker(Worker worker) {}
}
