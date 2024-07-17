package update.task_failure;

import io.temporal.activity.ActivityInterface;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.WorkflowUpdateException;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.UpdateValidatorMethod;
import io.temporal.workflow.Workflow;
import java.time.Duration;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Assertions;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {

  @UpdateMethod
  void execThrow();

  @UpdateMethod
  void updateWithValidator();

  @UpdateValidatorMethod(updateName = "updateWithValidator")
  void validatorThrow();

  @SignalMethod
  void finish();

  class Impl implements feature {

    private boolean doFinish = false;
    private static final AtomicInteger retryCount = new AtomicInteger();

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public void execThrow() {
      int c = retryCount.getAndIncrement();
      if (c < 3) {
        throw new IllegalArgumentException("simulated " + c);
      } else {
        throw ApplicationFailure.newFailure("simulated " + c, "Failure");
      }
    }

    @Override
    public void updateWithValidator() {}

    @Override
    public void validatorThrow() {
      throw new RuntimeException("bad validator");
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void workflowOptions(WorkflowOptions.Builder builder) {
      builder.setWorkflowTaskTimeout(Duration.ofSeconds(1));
    }

    @Override
    public Run execute(Runner runner) {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      // Check an update handler will retry runtime exception, but fail on a TemporalFailure
      try {
        stub.execThrow();
        Assertions.fail("unreachable");
      } catch (WorkflowUpdateException e) {
        Assertions.assertTrue(e.getCause() instanceof ApplicationFailure);
        Assertions.assertEquals("Failure", ((ApplicationFailure) e.getCause()).getType());
        Assertions.assertEquals(
            "message='simulated 3', type='Failure', nonRetryable=false", e.getCause().getMessage());
      }

      // Check an update handle validator will fail on any exception
      try {
        stub.updateWithValidator();
        Assertions.fail("unreachable");
      } catch (WorkflowUpdateException e) {
        Assertions.assertTrue(e.getCause() instanceof RuntimeException);
        Assertions.assertEquals(
            "message='bad validator', type='java.lang.RuntimeException', nonRetryable=false",
            e.getCause().getMessage());
      }

      stub.finish();
      return run;
    }
  }
}
