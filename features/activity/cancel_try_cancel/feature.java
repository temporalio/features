package activity.cancel_try_cancel;

import io.temporal.activity.Activity;
import io.temporal.activity.ActivityCancellationType;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.client.ActivityCanceledException;
import io.temporal.client.WorkflowClient;
import io.temporal.common.RetryOptions;
import io.temporal.failure.ActivityFailure;
import io.temporal.failure.ApplicationFailure;
import io.temporal.failure.CanceledFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.*;

import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicReference;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  @SignalMethod
  void activityResult(String result);

  @ActivityMethod
  void cancellableActivity();

  class Impl implements feature {
    private CompletablePromise<String> activityResult;

    @Override
    public void workflow() {
      activityResult = Workflow.newPromise();
      var activities = activities(feature.class, builder -> builder
              .setScheduleToCloseTimeout(Duration.ofMinutes(1))
              .setHeartbeatTimeout(Duration.ofSeconds(5))
              // Disable retry
              .setRetryOptions(RetryOptions.newBuilder().setMaximumAttempts(1).build())
              .setCancellationType(ActivityCancellationType.TRY_CANCEL));

      // Start cancellable activity
      var activityPromise = new AtomicReference<Promise<Void>>();
      var scope = Workflow.newCancellationScope(() ->
              activityPromise.set(Async.procedure(activities::cancellableActivity))
      );
      scope.run();

      // Sleep for short time (force task turnover)
      Workflow.sleep(1);

      // Cancel the activity and confirm it gets cancelled
      scope.cancel();
      try {
        activityPromise.get().get();
        throw ApplicationFailure.newFailure("No error", "NoError");
      } catch (ActivityFailure e) {
        if (!(e.getCause() instanceof CanceledFailure)) {
          throw e;
        }
      }

      // Confirm activity was cancelled
      try {
        var result = activityResult.get(10, TimeUnit.SECONDS);
        if (!"cancelled".equals(result)) {
          throw ApplicationFailure.newFailure("Expected cancelled, got: " + result, "BadResult");
        }
      } catch (TimeoutException e) {
        throw ApplicationFailure.newFailureWithCause("Timeout", "Timeout", e);
      }
    }

    @Override
    public void activityResult(String result) {
      activityResult.complete(result);
    }

    private WorkflowClient client;

    @Override
    public Run execute(Runner runner) throws Exception {
      client = runner.client;
      return feature.super.execute(runner);
    }

    @Override
    public void cancellableActivity() {
      Objects.requireNonNull(client);

      // Heartbeat every second for a minute
      var result = "timeout";
      for (int i = 0; i < 60; i++) {
        try {
          Thread.sleep(1000);
        } catch (InterruptedException e) {
          throw Activity.wrap(e);
        }
        try {
          Activity.getExecutionContext().heartbeat(null);
        } catch (ActivityCanceledException e) {
          result = "cancelled";
          break;
        }
      }

      // Send signal result
      client.newWorkflowStub(feature.class,
              Activity.getExecutionContext().getInfo().getWorkflowId()).activityResult(result);
    }
  }
}
