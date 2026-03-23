package activity.shutdown;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.api.enums.v1.TimeoutType;
import io.temporal.common.RetryOptions;
import io.temporal.failure.ActivityFailure;
import io.temporal.failure.ApplicationFailure;
import io.temporal.failure.TimeoutFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.WorkerFactory;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.concurrent.TimeUnit;

@ActivityInterface
public interface feature extends Feature {

  @WorkflowInterface
  interface ShutdownWorkflow {
    @WorkflowMethod
    String workflow();
  }

  @ActivityMethod
  void cancelSuccess();

  @ActivityMethod
  void cancelFailure();

  @ActivityMethod
  void cancelIgnore();

  class Impl implements feature, ShutdownWorkflow {
    private WorkerFactory initialFactory;

    @Override
    public String workflow() {
      var gracefulActivities =
          activities(
              feature.class,
              builder ->
                  builder
                      .setScheduleToCloseTimeout(Duration.ofSeconds(30))
                      .setRetryOptions(RetryOptions.newBuilder().setMaximumAttempts(1).build()));
      var ignoringActivities =
          activities(
              feature.class,
              builder ->
                  builder
                      .setScheduleToCloseTimeout(Duration.ofMillis(300))
                      .setRetryOptions(RetryOptions.newBuilder().setMaximumAttempts(1).build()));

      gracefulActivities.cancelSuccess();

      try {
        gracefulActivities.cancelFailure();
        throw ApplicationFailure.newFailure("expected failure", "NoError");
      } catch (ActivityFailure e) {
        if (!(e.getCause() instanceof ApplicationFailure)
            || !e.getCause().getMessage().contains("worker is shutting down")) {
          throw e;
        }
      }

      try {
        ignoringActivities.cancelIgnore();
        throw ApplicationFailure.newFailure("expected timeout", "NoError");
      } catch (ActivityFailure e) {
        if (e.getCause() instanceof TimeoutFailure) {
          TimeoutFailure t = (TimeoutFailure) e.getCause();
          if (t.getTimeoutType() == TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_CLOSE) {
            return "done";
          }
        }
        throw e;
      }
    }

    @Override
    public void cancelSuccess() {
      waitForShutdown();
    }

    @Override
    public void cancelFailure() {
      waitForShutdown();
      throw ApplicationFailure.newFailure("worker is shutting down", "Shutdown");
    }

    @Override
    public void cancelIgnore() {
      try {
        Thread.sleep(15000);
      } catch (InterruptedException e) {
        // Ignore
      }
    }

    // TODO: Use Activity context once https://github.com/temporalio/sdk-java/issues/1005 is
    // implemented
    private void waitForShutdown() {
      while (!initialFactory.isShutdown()) {
        try {
          Thread.sleep(100);
        } catch (InterruptedException e) {
          // ignore
        }
      }
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      this.initialFactory = runner.getWorkerFactory();
      var run = runner.executeSingleParameterlessWorkflow();

      // Wait for activity task to be scheduled
      runner.waitForActivityTaskScheduled(run, Duration.ofSeconds(5));

      runner.getWorkerFactory().shutdown();
      runner.getWorkerFactory().awaitTermination(1, TimeUnit.SECONDS);
      runner.restartWorker();
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals("done", runner.waitForRunResult(run, String.class));
    }
  }
}
