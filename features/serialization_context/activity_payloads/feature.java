package serialization_context.activity_payloads;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.temporal.activity.Activity;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.RetryOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String WORKFLOW_INPUT = "hello";
  String HEARTBEAT_DATA = "beat";

  @WorkflowMethod
  String workflow(String input);

  @ActivityInterface
  interface Activities {
    @ActivityMethod
    String activityWithHeartbeat(String input);

    /** Fails its first attempt so the second one has to decode the heartbeat details. */
    class Impl implements Activities {
      @Override
      public String activityWithHeartbeat(String input) {
        var context = Activity.getExecutionContext();
        if (context.getInfo().getAttempt() == 1) {
          context.heartbeat(HEARTBEAT_DATA);
          throw ApplicationFailure.newFailure(
              "retrying to read back heartbeat details", "RetryError");
        }
        return input + "|" + context.getHeartbeatDetails(String.class).orElse("");
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerActivitiesImplementations(new Activities.Impl());
    }

    @Override
    public String workflow(String input) {
      var activities =
          Workflow.newActivityStub(
              Activities.class,
              ActivityOptions.newBuilder()
                  .setStartToCloseTimeout(Duration.ofSeconds(10))
                  .setHeartbeatTimeout(Duration.ofSeconds(5))
                  .setRetryOptions(
                      RetryOptions.newBuilder()
                          .setInitialInterval(Duration.ofMillis(1))
                          .setMaximumAttempts(2)
                          .build())
                  .build());
      return activities.activityWithHeartbeat(input);
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(null, WORKFLOW_INPUT);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(
          WORKFLOW_INPUT + "|" + HEARTBEAT_DATA, runner.waitForRunResult(run, String.class));

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

      var expected =
          SerContext.activitySignature(
              runner.config.namespace,
              run.execution.getWorkflowId(),
              started.getWorkflowType().getName(),
              scheduled.getActivityType().getName(),
              scheduled.getTaskQueue().getName(),
              false);
      assertEquals(expected, SerContext.firstSignature(scheduled.getInput()));

      var completed =
          SerContext.findEvent(
                  history, "ActivityTaskCompleted", e -> e.hasActivityTaskCompletedEventAttributes())
              .getActivityTaskCompletedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(completed.getResult()));

      var workflowCompleted =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionCompleted",
                  e -> e.hasWorkflowExecutionCompletedEventAttributes())
              .getWorkflowExecutionCompletedEventAttributes();
      assertEquals(
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId()),
          SerContext.firstSignature(workflowCompleted.getResult()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
