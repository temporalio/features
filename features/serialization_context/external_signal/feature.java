package serialization_context.external_signal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.Optional;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String SIGNAL_DATA = "signaled";

  @WorkflowMethod
  String workflow(String targetId);

  @WorkflowInterface
  interface Receiver {
    @WorkflowMethod
    String execute();

    @SignalMethod(name = "external")
    void external(String data);

    class Impl implements Receiver {
      private String received;

      @Override
      public String execute() {
        Workflow.await(() -> received != null);
        return received;
      }

      @Override
      public void external(String data) {
        this.received = data;
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(Receiver.Impl.class);
    }

    /**
     * Signals another running workflow. The payload is serialized with the target's workflow ID,
     * not this workflow's own ID.
     */
    @Override
    public String workflow(String targetId) {
      Workflow.newExternalWorkflowStub(Receiver.class, targetId).external(SIGNAL_DATA);
      return targetId;
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    private static String receiverId(Runner runner) {
      return runner.config.taskQueue + "-receiver";
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var receiverOptions =
          WorkflowOptions.newBuilder()
              .setWorkflowId(receiverId(runner))
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .build();
      var receiver = runner.client.newWorkflowStub(Receiver.class, receiverOptions);
      var receiverExecution = WorkflowClient.start(receiver::execute);

      var run = runner.executeSingleWorkflow(null, receiverId(runner));

      var receiverStub =
          runner.client.newUntypedWorkflowStub(receiverExecution, Optional.empty());
      assertEquals(SIGNAL_DATA, receiverStub.getResult(String.class));
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var target = receiverId(runner);
      assertEquals(target, runner.waitForRunResult(run, String.class));

      var expected = SerContext.workflowSignature(runner.config.namespace, target);
      assertNotEquals(
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId()),
          expected);

      var initiated =
          SerContext.findEvent(
                  runner.getWorkflowHistory(run),
                  "SignalExternalWorkflowExecutionInitiated",
                  e -> e.hasSignalExternalWorkflowExecutionInitiatedEventAttributes())
              .getSignalExternalWorkflowExecutionInitiatedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(initiated.getInput()));

      var receiverRun =
          new Run(run.method, WorkflowExecution.newBuilder().setWorkflowId(target).build());
      var signaled =
          SerContext.findEvent(
                  runner.getWorkflowHistory(receiverRun),
                  "WorkflowExecutionSignaled",
                  e -> e.hasWorkflowExecutionSignaledEventAttributes())
              .getWorkflowExecutionSignaledEventAttributes();
      assertEquals(expected, SerContext.firstSignature(signaled.getInput()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
