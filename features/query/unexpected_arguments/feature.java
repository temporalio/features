package query.unexpected_arguments;

import io.temporal.sdkfeatures.Assertions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;

public interface feature extends Feature, SimpleWorkflow {
  @QueryMethod
  String theQuery(int arg);

  @SignalMethod
  void finish();

  class Impl implements feature {
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public String theQuery(int arg) {
      return String.format("got %d", arg);
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      // Java doesn't reject anything.
      var stub = runner.client.newUntypedWorkflowStub(run.execution.getWorkflowId());

      var res = stub.query("theQuery", String.class, 123);
      Assertions.assertEquals(res, "got 123");

      // Silently drops extra arg
      res = stub.query("theQuery", String.class, 123, true);
      Assertions.assertEquals(res, "got 123");

      // Assumes default value for int
      res = stub.query("theQuery", String.class);
      Assertions.assertEquals(res, "got 0");

      stub.signal("finish");
      runner.waitForRunResult(run);
    }
  }
}
