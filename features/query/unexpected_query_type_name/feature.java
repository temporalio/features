package query.unexpected_query_type_name;

import io.temporal.client.WorkflowQueryException;
import io.temporal.sdkfeatures.Assertions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;

public interface feature extends Feature, SimpleWorkflow {
  @SignalMethod
  void finish();

  class Impl implements feature {
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var stub = runner.client.newUntypedWorkflowStub(run.execution.getWorkflowId());

      Assertions.assertThrows(WorkflowQueryException.class, () -> stub.query("nonexistent", null));

      stub.signal("finish");
      runner.waitForRunResult(run);
    }
  }
}
