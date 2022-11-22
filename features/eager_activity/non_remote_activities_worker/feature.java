package eager_activity.non_remote_activities_worker;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.api.enums.v1.TimeoutType;
import io.temporal.failure.ActivityFailure;
import io.temporal.failure.TimeoutFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.worker.WorkerOptions;

import java.time.Duration;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  // Start a worker with activities registered and non-local activities disabled
  @Override
  default void workerOptions(WorkerOptions.Builder builder) {
    builder.setLocalActivityWorkerOnly(true);
  }

  @ActivityMethod
  void dummy();

  class Impl implements feature {
    @Override
    public void workflow() {
      // Run a workflow that schedules a single activity with short schedule-to-close
      // timeout
      var activities = activities(feature.class, builder -> builder
          // Pick a long enough timeout for busy CI but not too long to get feedback
          // quickly
          .setScheduleToCloseTimeout(Duration.ofSeconds(3)));

      try {
        activities.dummy();
        throw new RuntimeException("Expected activity to throw");
      } catch (ActivityFailure e) {
        // Catch activity failure in the workflow, check that it is caused by
        // schedule-to-start timeout
        if (e.getCause() instanceof TimeoutFailure) {
          var timeoutFailure = ((TimeoutFailure) e.getCause());
          if (timeoutFailure.getTimeoutType() == TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_CLOSE) {
            return;
          }
        }
        throw e;
      }
    }

    @Override
    public void dummy() {
    }
  }
}
