package signal.external;

import io.temporal.activity.ActivityInterface;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

import static org.junit.jupiter.api.Assertions.assertEquals;


@WorkflowInterface
public interface feature extends Feature {
  @SignalMethod
  void externalSignal(String result);

  @WorkflowMethod
  public String workflow();

  class Impl implements feature {
    private static final String SIGNAL_DATA = "Signaled!";
    private String workflowResult;

    @Override
    public String workflow() {
      Workflow.await(() -> workflowResult != null);
      return workflowResult;
    }

    @Override
    public void externalSignal(String result) {
      workflowResult = result;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());
      stub.externalSignal(SIGNAL_DATA);
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var resultStr = runner.waitForRunResult(run, String.class);
      assertEquals(resultStr, SIGNAL_DATA);
    }
  }
}
