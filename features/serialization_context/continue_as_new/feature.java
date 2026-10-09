package serialization_context.continue_as_new;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String FINAL_RESULT = "done";

  @WorkflowMethod
  String workflow(int remaining);

  class Impl implements feature {

    @Override
    public String workflow(int remaining) {
      if (remaining > 0) {
        Workflow.continueAsNew(remaining - 1);
      }
      return FINAL_RESULT;
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(null, 1);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(FINAL_RESULT, runner.waitForRunResult(run, String.class));

      // Continue-as-new keeps the workflow ID, so both runs share the context.
      var expected =
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId());

      var continued =
          SerContext.findEvent(
                  runner.getWorkflowHistory(run),
                  "WorkflowExecutionContinuedAsNew",
                  e -> e.hasWorkflowExecutionContinuedAsNewEventAttributes())
              .getWorkflowExecutionContinuedAsNewEventAttributes();
      assertEquals(expected, SerContext.firstSignature(continued.getInput()));

      var lastRun =
          new Run(
              run.method,
              WorkflowExecution.newBuilder().setWorkflowId(run.execution.getWorkflowId()).build());
      var lastRunHistory = runner.getWorkflowHistory(lastRun);

      var started =
          SerContext.findEvent(
                  lastRunHistory,
                  "WorkflowExecutionStarted",
                  e -> e.hasWorkflowExecutionStartedEventAttributes())
              .getWorkflowExecutionStartedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(started.getInput()));

      var completed =
          SerContext.findEvent(
                  lastRunHistory,
                  "WorkflowExecutionCompleted",
                  e -> e.hasWorkflowExecutionCompletedEventAttributes())
              .getWorkflowExecutionCompletedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(completed.getResult()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
