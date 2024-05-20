package update.deduplication;

import io.temporal.client.UpdateHandle;
import io.temporal.client.UpdateOptions;
import io.temporal.client.WorkflowUpdateStage;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.*;
import java.time.Duration;
import java.util.Optional;
import org.junit.jupiter.api.Assertions;

@WorkflowInterface
public interface feature extends Feature {
  Duration SLEEP_TIMEOUT = Duration.ofSeconds(1);
  String REUSED_UPDATE_ID = "reused_update_id";

  @UpdateMethod
  int incrementCount();

  @SignalMethod
  void finish();

  @WorkflowMethod
  int workflow();

  class Impl implements feature {
    private boolean doFinish = false;
    private int counter = 0;

    @Override
    public int workflow() {
      Workflow.await(() -> this.doFinish);
      return counter;
    }

    @Override
    public int incrementCount() {
      counter += 1;
      // Check that deduplication does not need completion
      Workflow.sleep(SLEEP_TIMEOUT);
      return counter;
    }

    @Override
    public void finish() {
      doFinish = true;
    }

    public static long getCountCompletedUpdates(Runner runner, Run run) throws Exception {
      var history = runner.getWorkflowHistory(run);
      return history.getEventsList().stream()
          .filter(e -> e.hasWorkflowExecutionUpdateCompletedEventAttributes())
          .count();
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      var untypedStub =
          runner.client.newUntypedWorkflowStub(
              run.execution.getWorkflowId(),
              Optional.of(run.execution.getRunId()),
              Optional.empty());

      var updateOptions =
          UpdateOptions.newBuilder(Integer.class)
              .setUpdateName("incrementCount")
              .setUpdateId(REUSED_UPDATE_ID)
              .setWaitPolicy(WorkflowUpdateStage.ACCEPTED)
              .setFirstExecutionRunId(run.execution.getRunId())
              .build();

      UpdateHandle<Integer> handle1 = untypedStub.startUpdate(updateOptions);
      UpdateHandle<Integer> handle2 = untypedStub.startUpdate(updateOptions);

      Assertions.assertEquals(1, handle1.getResultAsync().get());
      Assertions.assertEquals(1, handle2.getResultAsync().get());

      Assertions.assertEquals(1, getCountCompletedUpdates(runner, run));
      stub.finish();
      return run;
    }
  }
}
