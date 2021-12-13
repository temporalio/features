package io.temporal.sdkfeatures;

import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.worker.WorkerOptions;
import io.temporal.workflow.Workflow;

import java.util.function.Consumer;

public interface Feature {

  @SuppressWarnings("unchecked")
  default <T> T activities(Class<T> activityIface, Consumer<ActivityOptions.Builder> builderFunc) {
    var builder = ActivityOptions.newBuilder();
    builderFunc.accept(builder);
    return (T) Workflow.newActivityStub(activityIface, builder.build());
  }

  default void workflowServiceOptions(WorkflowServiceStubsOptions.Builder builder) { }

  default void workflowClientOptions(WorkflowClientOptions.Builder builder) { }

  default void workerFactoryOptions(WorkerFactoryOptions.Builder builder) { }

  default void workerOptions(WorkerOptions.Builder builder) { }

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
}
