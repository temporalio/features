package serialization_context.failure;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.fail;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.RetryOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.worker.Worker;
import io.temporal.workflow.Workflow;
import java.time.Duration;
import serialization_context.sercontext.SerContext;

public interface feature extends Feature, SimpleWorkflow {

  String ACTIVITY_ERROR_MESSAGE = "activity failed";
  String WORKFLOW_ERROR_MESSAGE = "workflow failed";

  @ActivityInterface
  interface Activities {
    @ActivityMethod
    void failingActivity();

    class Impl implements Activities {
      @Override
      public void failingActivity() {
        throw ApplicationFailure.newNonRetryableFailure(ACTIVITY_ERROR_MESSAGE, "ActivityError");
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerActivitiesImplementations(new Activities.Impl());
    }

    /**
     * Lets an activity fail and then fails itself, so that both an activity scoped and a workflow
     * scoped failure conversion are recorded.
     */
    @Override
    public void workflow() {
      var activities =
          Workflow.newActivityStub(
              Activities.class,
              ActivityOptions.newBuilder()
                  .setStartToCloseTimeout(Duration.ofSeconds(10))
                  .setRetryOptions(RetryOptions.newBuilder().setMaximumAttempts(1).build())
                  .build());
      try {
        activities.failingActivity();
      } catch (Exception e) {
        throw ApplicationFailure.newFailure(WORKFLOW_ERROR_MESSAGE, "WorkflowError");
      }
      throw ApplicationFailure.newFailure("expected the activity to fail", "WorkflowError");
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      try {
        runner.waitForRunResult(run);
        fail("expected the workflow to fail");
      } catch (Exception e) {
        // Expected.
      }

      var history = runner.getWorkflowHistory(run);
      var started =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionStarted",
                  e -> e.hasWorkflowExecutionStartedEventAttributes())
              .getWorkflowExecutionStartedEventAttributes();
      var scheduled =
          SerContext.findEvent(
                  history, "ActivityTaskScheduled", e -> e.hasActivityTaskScheduledEventAttributes())
              .getActivityTaskScheduledEventAttributes();

      var activityFailed =
          SerContext.findEvent(
                  history, "ActivityTaskFailed", e -> e.hasActivityTaskFailedEventAttributes())
              .getActivityTaskFailedEventAttributes();
      assertEquals(
          SerContext.activitySignature(
              runner.config.namespace,
              run.execution.getWorkflowId(),
              started.getWorkflowType().getName(),
              scheduled.getActivityType().getName(),
              scheduled.getTaskQueue().getName(),
              false),
          activityFailed.getFailure().getSource());

      var workflowFailed =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionFailed",
                  e -> e.hasWorkflowExecutionFailedEventAttributes())
              .getWorkflowExecutionFailedEventAttributes();
      assertEquals(
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId()),
          workflowFailed.getFailure().getSource());
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
