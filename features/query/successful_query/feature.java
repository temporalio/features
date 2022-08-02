package query.successful_query;

import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import java.util.Optional;
import org.junit.jupiter.api.Assertions;

public interface feature extends Feature, SimpleWorkflow {
  @QueryMethod
  int counterQuery();

  @SignalMethod
  void incCounter();

  @SignalMethod
  void finish();

  class Impl implements feature {
    private int counter = 0;
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public int counterQuery() {
      return this.counter;
    }

    @Override
    public void incCounter() {
      this.counter += 1;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var stub = runner.client.newWorkflowStub(feature.class,
          run.execution.getWorkflowId(), Optional.of(run.execution.getRunId()));
      var q1 = stub.counterQuery();
      Assertions.assertEquals(0, q1);
      stub.incCounter();
      var q2 = stub.counterQuery();
      Assertions.assertEquals(1, q2);
      stub.incCounter();
      stub.incCounter();
      stub.incCounter();
      var q3 = stub.counterQuery();
      Assertions.assertEquals(4, q3);
      stub.finish();
      runner.waitForRunResult(run);
    }
  }
}
