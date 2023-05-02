package update.self;

import io.temporal.activity.Activity;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.client.WorkflowClient;
import io.temporal.sdkfeatures.Assertions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.Objects;

@ActivityInterface
public interface feature extends Feature {
  @WorkflowInterface
  interface SelfUpdateWorkflow {
    @WorkflowMethod
    String workflow();

    @UpdateMethod
    void update();
  }

  String INITIAL_STATE = "Not signaled";

  String FINAL_STATE = "Signaled!";

  @ActivityMethod
  void selfUpdate();

  class Impl implements feature, SelfUpdateWorkflow {
    private String state = INITIAL_STATE;
    private WorkflowClient client;

    @Override
    public void selfUpdate() {
      Objects.requireNonNull(client);
      client
          .newWorkflowStub(
              feature.SelfUpdateWorkflow.class,
              Activity.getExecutionContext().getInfo().getWorkflowId())
          .update();
    }

    @Override
    public String workflow() {
      var activities =
          activities(
              feature.class, builder -> builder.setScheduleToCloseTimeout(Duration.ofSeconds(5)));

      activities.selfUpdate();
      return state;
    }

    @Override
    public void update() {
      state = FINAL_STATE;
    }

    @Override
    public Run execute(Runner runner) {
      runner.skipIfUpdateNotSupported();

      client = runner.client;
      return runner.executeSingleParameterlessWorkflow();
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(FINAL_STATE, runner.waitForRunResult(run));
    }
  }
}
