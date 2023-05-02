package update.validation_replay;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.UpdateValidatorMethod;
import io.temporal.workflow.Workflow;
import java.time.Duration;
import org.junit.jupiter.api.Assertions;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  int activityResult = 6;

  @ActivityMethod
  int someActivity();

  @UpdateMethod
  int update(int i);

  @UpdateValidatorMethod(updateName = "update")
  void validate(int i);

  @SignalMethod
  void finish();

  class Impl implements feature {

    private static int validationCounter = 0;
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public int someActivity() {
      return activityResult;
    }

    @Override
    public int update(int i) {
      var activities =
          activities(
              update.activities.feature.class,
              builder -> builder.setScheduleToCloseTimeout(Duration.ofSeconds(5)));
      return activities.someActivity();
    }

    @Override
    public void validate(int i) {
      if (validationCounter++ > 0) {
        throw new IllegalArgumentException("failing validation");
      }
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      Assertions.assertEquals(activityResult, stub.update(1));

      stub.finish();
      runner.requireNoUpdateRejectedEvents(run);

      return run;
    }
  }
}
