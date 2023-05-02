package update.activities;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.*;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Assertions;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  int activityResult = 6;
  int activityCount = 5;

  @ActivityMethod
  int someActivity();

  @UpdateMethod
  int update();

  @SignalMethod
  void finish();

  class Impl implements feature {

    private boolean doFinish = false;

    @Override
    public int someActivity() {
      return activityResult;
    }

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public int update() {
      var activities =
          activities(
              feature.class, builder -> builder.setScheduleToCloseTimeout(Duration.ofSeconds(5)));
      List<String> results = new ArrayList();

      List<Promise<Integer>> promiseList = new ArrayList<>();
      var total = 0;
      for (int i = 0; i < activityCount; i++) {
        promiseList.add(Async.function(activities::someActivity));
      }

      // Invoke all activities in parallel. Wait for all to complete
      Promise.allOf(promiseList).get();

      // Loop through promises and total results
      for (Promise<Integer> promise : promiseList) {
        if (promise.getFailure() == null) {
          total += promise.get();
        }
      }

      return total;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public Run execute(Runner runner) {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      Integer updateResult = stub.update();
      Assertions.assertEquals(activityResult * activityCount, updateResult);

      stub.finish();
      return run;
    }
  }
}
