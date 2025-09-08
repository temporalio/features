package activity.basic_no_workflow_timeout;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.SimpleWorkflow;
import java.time.Duration;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  @ActivityMethod
  String echo();

  // Start a worker by disabling workflow cache.
  // This effectively tells the server to not return the next workflow task.
  @Override
  default void workerOptions(WorkerOptions.Builder builder) {
    builder.setMaxWorkflowCacheSize(0);
  }

  class Impl implements feature {
    @Override
    public void workflow() {
      var activities =
          activities(
              feature.class, builder -> builder.setStartToCloseTimeout(Duration.ofMinutes(1)));

      activities.echo();

      var activitiesSched2Close =
          activities(
              feature.class, builder -> builder.setScheduleToCloseTimeout(Duration.ofMinutes(1)));

      activitiesSched2Close.echo();
    }

    @Override
    public String echo() {
      return "hi";
    }
  }
}
