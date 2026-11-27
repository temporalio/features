package update.rejected_update_worker_restart;

import io.temporal.client.WorkflowUpdateException;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.UpdateValidatorMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Assertions;

@WorkflowInterface
public interface feature extends Feature {

  @WorkflowMethod
  int workflow();

  @UpdateMethod
  int add(int value);

  @UpdateValidatorMethod(updateName = "add")
  void validate(int value);

  @SignalMethod
  void finish();

  class Impl implements feature {
    private int counter;
    private boolean finished;

    @Override
    public int workflow() {
      Workflow.await(() -> finished);
      return counter;
    }

    @Override
    public int add(int value) {
      counter += value;
      return counter;
    }

    @Override
    public void validate(int value) {
      if (value < 0) {
        throw new IllegalArgumentException("negative value: " + value);
      }
    }

    @Override
    public void finish() {
      finished = true;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      runner.waitForEvent(
          run, event -> event.hasWorkflowTaskCompletedEventAttributes(), Duration.ofSeconds(10));
      var before = runner.getWorkflowHistory(run);
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      Assertions.assertThrows(WorkflowUpdateException.class, () -> stub.add(-1));
      Assertions.assertEquals(before, runner.getWorkflowHistory(run), "rejection changed history");

      runner.getWorkerFactory().shutdown();
      runner.getWorkerFactory().awaitTermination(5, TimeUnit.SECONDS);
      runner.restartWorker();

      Assertions.assertEquals(1, stub.add(1));
      stub.finish();
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(1, runner.waitForRunResult(run, Integer.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      runner.checkCurrentAndPastHistories(run);
      var history = runner.getWorkflowHistory(run);
      long accepted =
          history.getEventsList().stream()
              .filter(event -> event.hasWorkflowExecutionUpdateAcceptedEventAttributes())
              .count();
      long completed =
          history.getEventsList().stream()
              .filter(event -> event.hasWorkflowExecutionUpdateCompletedEventAttributes())
              .count();
      long rejected =
          history.getEventsList().stream()
              .filter(event -> event.hasWorkflowExecutionUpdateRejectedEventAttributes())
              .count();
      Assertions.assertEquals(1, accepted, "expected one accepted Update");
      Assertions.assertEquals(1, completed, "expected one completed Update");
      Assertions.assertEquals(0, rejected, "rejected Update was recorded in history");
    }
  }
}
