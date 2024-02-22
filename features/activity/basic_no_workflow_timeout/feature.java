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

  class Impl implements feature {
    @Override
    public void workflow() {
      // Allow 4 retries with no backoff
      var activities =
          activities(
              feature.class, builder -> builder.setStartToCloseTimeout(Duration.ofMinutes(1)));

      // Execute activity
      activities.echo();
    }

    @Override
    public String echo() {
      return "hi";
    }
  }

  @Override
  default void workflowOptions(WorkflowOptions.Builder builder) {
    builder.setWorkflowExecutionTimeout(Duration.ZERO);
  }
}
