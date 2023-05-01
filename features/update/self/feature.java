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
import update.updateutil.UpdateUtil;

import java.time.Duration;
import java.util.Objects;

@ActivityInterface
public interface feature extends Feature, SelfUpdateWorkflow {

  String INITIAL_STATE = "Not signaled";

  String FINAL_STATE = "Signaled!";

  @ActivityMethod
  void selfUpdate();

  @UpdateMethod()
  void update();


  class Impl implements feature {
    private String state = INITIAL_STATE;
    private WorkflowClient client;


    @Override
    public void selfUpdate() {
      Objects.requireNonNull(client);
      client.newWorkflowStub(feature.class, Activity.getExecutionContext().getInfo().getWorkflowId()).update();
    }


    @Override
    public String workflow() {
      var activities = activities(feature.class, builder -> builder
              .setScheduleToCloseTimeout(Duration.ofSeconds(5)));

      activities.selfUpdate();
      return state;
    }

    @Override
    public void update() {
      state = FINAL_STATE;
    }

    @Override
    public Run execute(Runner runner) {
      String reason = UpdateUtil.CheckServerSupportsUpdate(runner.client);
      if (!reason.isEmpty()) {
        runner.Skip(reason);
      }

      client = runner.client;
      return runner.executeSingleParameterlessWorkflow();
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(FINAL_STATE, runner.waitForRunResult(run));
    }
  }
}
