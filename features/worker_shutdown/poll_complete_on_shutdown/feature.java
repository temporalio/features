package worker_shutdown.poll_complete_on_shutdown;

import static org.junit.jupiter.api.Assertions.assertFalse;

import com.google.gson.JsonParser;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowOptions;
import io.temporal.common.RetryOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;

@ActivityInterface
public interface feature extends Feature {
  int WORKFLOW_COUNT = 5;

  @WorkflowInterface
  interface PollCompleteWorkflow {
    @WorkflowMethod
    void workflow();
  }

  @ActivityMethod
  void noop();

  class Impl implements feature, PollCompleteWorkflow {
    @Override
    public Run execute(Runner runner) throws Exception {
      List<Run> runs = new ArrayList<>();
      try {
        for (int i = 0; i < WORKFLOW_COUNT; i++) {
          runs.add(
              runner.executeSingleWorkflow(
                  WorkflowOptions.newBuilder()
                      .setTaskQueue(runner.config.taskQueue)
                      .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
                      .setWorkflowTaskTimeout(Duration.ofSeconds(5))
                      .build()));
        }
        for (Run run : runs) {
          runner.waitForActivityTaskScheduled(run, Duration.ofSeconds(10));
        }

        long start = System.nanoTime();
        runner.getWorkerFactory().shutdown();
        runner.getWorkerFactory().awaitTermination(5, TimeUnit.SECONDS);
        Duration elapsed = Duration.ofNanos(System.nanoTime() - start);
        if (!runner.getWorkerFactory().isTerminated()
            || elapsed.compareTo(Duration.ofSeconds(5)) > 0) {
          throw new AssertionError("worker shutdown took " + elapsed);
        }

        if (expectWorkerPollCompleteOnShutdown()) {
          for (Run run : runs) {
            assertNoWorkflowTaskProblems(runner, run);
          }
        }
        return null;
      } finally {
        for (Run run : runs) {
          try {
            runner.client
                .newUntypedWorkflowStub(run.execution, java.util.Optional.empty())
                .terminate("feature cleanup");
          } catch (Exception ignored) {
            // Ignore cleanup races.
          }
        }
      }
    }

    @Override
    public void workflow() {
      var activities =
          activities(
              feature.class,
              builder ->
                  builder
                      .setScheduleToCloseTimeout(Duration.ofSeconds(10))
                      .setStartToCloseTimeout(Duration.ofSeconds(5))
                      .setRetryOptions(RetryOptions.newBuilder().setMaximumAttempts(1).build()));
      while (true) {
        Workflow.sleep(Duration.ofMillis(20));
        activities.noop();
      }
    }

    @Override
    public void noop() {}

    private static void assertNoWorkflowTaskProblems(Runner runner, Run run) throws Exception {
      var history = runner.getWorkflowHistory(run);
      assertFalse(
          history.getEventsList().stream()
              .anyMatch(
                  event ->
                      event.getEventType() == EventType.EVENT_TYPE_WORKFLOW_TASK_FAILED
                          || event.getEventType() == EventType.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT),
          "workflow task failed or timed out");
    }

    private static boolean expectWorkerPollCompleteOnShutdown() {
      var capabilitiesJson = System.getenv("FEATURE_NAMESPACE_CAPABILITIES");
      if (capabilitiesJson == null || capabilitiesJson.isEmpty()) {
        throw new IllegalStateException("FEATURE_NAMESPACE_CAPABILITIES is required");
      }
      var capabilities = JsonParser.parseString(capabilitiesJson).getAsJsonObject();
      if (!capabilities.has("workerPollCompleteOnShutdown")) {
        throw new IllegalStateException(
            "FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown");
      }
      return capabilities.get("workerPollCompleteOnShutdown").getAsBoolean();
    }
  }
}
