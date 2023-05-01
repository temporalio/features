package update.non_durable_reject;

import io.temporal.client.WorkflowUpdateException;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.*;
import org.junit.jupiter.api.Assertions;
import update.updateutil.UpdateUtil;

@WorkflowInterface
public interface feature extends Feature {
  int step = 2;
  int count  = 5;

  @WorkflowMethod
  Integer workflow();

  @UpdateMethod()
  int update(int i);

  @UpdateValidatorMethod(updateName = "update")
  void validate(int i);

  @SignalMethod
  void finish();

  class Impl implements feature {

    private int counter = 0;
    private boolean doFinish = false;

    @Override
    public Integer workflow() {
      Workflow.await(() -> this.doFinish);
      return counter;
    }

    @Override
    public int update(int i) {
      counter += i;
      return counter;
    }

    @Override
    public void validate(int i) {
      if (i < 0) {
        throw new IllegalArgumentException("expected non-negative value " + i);
      }
    }
    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public Run execute(Runner runner) {
      String reason = UpdateUtil.CheckServerSupportsUpdate(runner.client);
      if (!reason.isEmpty()) {
        runner.Skip(reason);
      }

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      Assertions.assertThrows(WorkflowUpdateException.class, () -> stub.update(-1));
      for (int i = 0; i < count; i++) {
        stub.update(step);
        Assertions.assertThrows(WorkflowUpdateException.class, () -> stub.update(-1));
      }

      stub.finish();
      UpdateUtil.RequireNoUpdateRejectedEvents(runner, run);

      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(step*count, runner.waitForRunResult(run, Integer.class));
    }
  }
}
