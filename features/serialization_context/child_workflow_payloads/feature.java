package serialization_context.child_workflow_payloads;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.ChildWorkflowOptions;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String WORKFLOW_INPUT = "hello";
  String CHILD_ID_SUFFIX = "_child";
  String CHILD_RESULT_TAG = "|child";

  @WorkflowMethod
  String workflow(String input);

  @WorkflowInterface
  interface ChildWorkflow {
    @WorkflowMethod
    String execute(String input);

    class Impl implements ChildWorkflow {
      @Override
      public String execute(String input) {
        return input + CHILD_RESULT_TAG;
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(ChildWorkflow.Impl.class);
    }

    @Override
    public String workflow(String input) {
      var child =
          Workflow.newChildWorkflowStub(
              ChildWorkflow.class,
              ChildWorkflowOptions.newBuilder()
                  .setWorkflowId(Workflow.getInfo().getWorkflowId() + CHILD_ID_SUFFIX)
                  .setWorkflowRunTimeout(Duration.ofMinutes(1))
                  .build());
      return child.execute(input);
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(null, WORKFLOW_INPUT);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(WORKFLOW_INPUT + CHILD_RESULT_TAG, runner.waitForRunResult(run, String.class));

      var childId = run.execution.getWorkflowId() + CHILD_ID_SUFFIX;
      // The child's payloads carry the child's own workflow ID, not the parent's.
      var expected = SerContext.workflowSignature(runner.config.namespace, childId);
      assertNotEquals(
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId()),
          expected);

      var parentHistory = runner.getWorkflowHistory(run);
      var initiated =
          SerContext.findEvent(
                  parentHistory,
                  "StartChildWorkflowExecutionInitiated",
                  e -> e.hasStartChildWorkflowExecutionInitiatedEventAttributes())
              .getStartChildWorkflowExecutionInitiatedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(initiated.getInput()));

      var childCompleted =
          SerContext.findEvent(
                  parentHistory,
                  "ChildWorkflowExecutionCompleted",
                  e -> e.hasChildWorkflowExecutionCompletedEventAttributes())
              .getChildWorkflowExecutionCompletedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(childCompleted.getResult()));

      var childRun =
          new Run(run.method, WorkflowExecution.newBuilder().setWorkflowId(childId).build());
      var childStarted =
          SerContext.findEvent(
                  runner.getWorkflowHistory(childRun),
                  "WorkflowExecutionStarted",
                  e -> e.hasWorkflowExecutionStartedEventAttributes())
              .getWorkflowExecutionStartedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(childStarted.getInput()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
