package child_workflow.result;

import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.Async;
import io.temporal.workflow.Promise;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import org.junit.jupiter.api.Assertions;

@WorkflowInterface
public interface feature extends Feature {

  @WorkflowInterface
  interface ChildWorkflow {
    @WorkflowMethod
    String execute(String input);

    class Impl implements ChildWorkflow {
      public String execute(String input) {
        return input;
      }
    }
  }

  @WorkflowMethod
  public String workflow();

  class Impl implements feature {
    private static final String CHILDWORKFLOW_INPUT = "test";

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(ChildWorkflow.Impl.class);
    }

    @Override
    public String workflow() {
      ChildWorkflow child = Workflow.newChildWorkflowStub(ChildWorkflow.class);
      Promise<String> result = Async.function(child::execute, CHILDWORKFLOW_INPUT);
      return result.get();
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var options =
          WorkflowOptions.newBuilder()
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .build();

      var methods = runner.featureInfo.metadata.getWorkflowMethods();

      var stub = runner.client.newWorkflowStub(feature.class, options);
      return new Run(methods.get(0), WorkflowClient.start(stub::workflow));
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var resultStr = runner.waitForRunResult(run, String.class);
      Assertions.assertEquals(CHILDWORKFLOW_INPUT, resultStr);
    }
  }
}
