package child_workflow.signal;

import io.temporal.sdkfeatures.Assertions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.Async;
import io.temporal.workflow.Promise;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface feature extends Feature {

  @WorkflowInterface
  interface ChildWorkflow {

    @WorkflowMethod
    String workflow();

    @SignalMethod
    void unblock(String message);

    class Impl implements ChildWorkflow {
      /*
       * A workflow that waits for a signal and returns the data received.
       */

      private String childWorkflowUnblockMessage;

      @Override
      public String workflow() {
        Workflow.await(() -> childWorkflowUnblockMessage != null);
        return childWorkflowUnblockMessage;
      }

      @Override
      public void unblock(String message) {
        childWorkflowUnblockMessage = message;
      }
    }
  }

  @WorkflowMethod
  String workflow();

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(ChildWorkflow.Impl.class);
    }

    private static final String UNBLOCK_MESSAGE = "unblock";

    /*
     * Parent workflow
     *
     * A workflow that starts a child workflow, unblocks it, and returns the
     * result of the child workflow.
     */

    @Override
    public String workflow() {
      ChildWorkflow child = Workflow.newChildWorkflowStub(ChildWorkflow.class);
      Promise<String> childResult = Async.function(child::workflow);
      child.unblock(UNBLOCK_MESSAGE);
      return childResult.get();
    }

    /* Test */

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleParameterlessWorkflow();
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var resultStr = runner.waitForRunResult(run, String.class);
      Assertions.assertEquals(UNBLOCK_MESSAGE, resultStr);
    }
  }
}
