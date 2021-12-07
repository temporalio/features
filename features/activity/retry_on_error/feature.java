package activity.retry_on_error;

import io.temporal.activity.Activity;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.common.RetryOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;

import java.time.Duration;

import static io.temporal.sdkfeatures.Assertions.assertActivityErrorMessage;
import static io.temporal.sdkfeatures.Assertions.fail;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  @ActivityMethod
  void alwaysFail();

  class Impl implements feature {
    @Override
    public void workflow() {
      // Allow 4 retries with no backoff
      var activities = activities(feature.class, builder -> builder
              .setScheduleToCloseTimeout(Duration.ofMinutes(1))
              .setRetryOptions(RetryOptions.newBuilder()
                      // Retry immediately
                      .setInitialInterval(Duration.ofNanos(1))
                      // Do not increase backoff retry each time
                      .setBackoffCoefficient(1)
                      // 5 total maximum attempts
                      .setMaximumAttempts(5)
                      .build()));

      // Execute activity
      activities.alwaysFail();
    }

    @Override
    public void alwaysFail() {
      throw new IllegalStateException("activity attempt " +
              Activity.getExecutionContext().getInfo().getAttempt() + " failed");
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      // This must be an error
      try {
        runner.waitForRunResult(run);
        fail("expected failure");
      } catch (Exception e) {
        assertActivityErrorMessage("activity attempt 5 failed", e);
      }
    }
  }
}
