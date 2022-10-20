package query.unexpected_return_type;

import io.temporal.client.WorkflowServiceException;
import io.temporal.common.converter.DataConverterException;
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
  String theQuery();

  @SignalMethod
  void finish();

  class Impl implements feature {
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public String theQuery() {
      return "hi bob";
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var stub = runner.client.newUntypedWorkflowStub(run.execution.getWorkflowId());

      var exc = Assertions.assertThrows(WorkflowServiceException.class,
          () -> stub.query("theQuery", Integer.class));
      Assertions.assertInstanceOf(DataConverterException.class, exc.getCause());

      stub.signal("finish");
      runner.waitForRunResult(run);
    }
  }
}
