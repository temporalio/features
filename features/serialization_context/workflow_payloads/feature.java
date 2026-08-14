package serialization_context.workflow_payloads;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.Collections;
import java.util.Optional;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String WORKFLOW_INPUT = "input";
  String MEMO_KEY = "ser-ctx-memo";
  String MEMO_VALUE = "memo";
  String QUERY_ARG = "query-";
  String UPDATE_ARG = "-update";
  String SIGNAL_DATA = "signal";

  @WorkflowMethod
  String workflow(String input);

  @SignalMethod(name = "append")
  void append(String data);

  @QueryMethod(name = "prefixed")
  String prefixed(String prefix);

  @UpdateMethod(name = "suffixed")
  String suffixed(String suffix);

  /**
   * Exercises every workflow scoped payload: input and result, a memo, a signal, a query and an
   * update.
   */
  class Impl implements feature {

    private String input = "";
    private String signaled;

    @Override
    public String workflow(String input) {
      this.input = input;
      Workflow.await(() -> signaled != null);
      return input + "|" + signaled;
    }

    @Override
    public void append(String data) {
      this.signaled = data;
    }

    @Override
    public String prefixed(String prefix) {
      return prefix + input;
    }

    @Override
    public String suffixed(String suffix) {
      return input + suffix;
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var options =
          WorkflowOptions.newBuilder()
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .setMemo(Collections.singletonMap(MEMO_KEY, MEMO_VALUE))
              .build();
      var run = runner.executeSingleWorkflow(options, WORKFLOW_INPUT);

      var stub = runner.client.newUntypedWorkflowStub(run.execution, Optional.empty());
      assertEquals(QUERY_ARG + WORKFLOW_INPUT, stub.query("prefixed", String.class, QUERY_ARG));
      assertEquals(WORKFLOW_INPUT + UPDATE_ARG, stub.update("suffixed", String.class, UPDATE_ARG));
      stub.signal("append", SIGNAL_DATA);
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(WORKFLOW_INPUT + "|" + SIGNAL_DATA, runner.waitForRunResult(run, String.class));

      var history = runner.getWorkflowHistory(run);
      var expected =
          SerContext.workflowSignature(runner.config.namespace, run.execution.getWorkflowId());

      var started =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionStarted",
                  e -> e.hasWorkflowExecutionStartedEventAttributes())
              .getWorkflowExecutionStartedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(started.getInput()));
      assertEquals(
          expected, SerContext.signatureOf(started.getMemo().getFieldsMap().get(MEMO_KEY)));

      var completed =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionCompleted",
                  e -> e.hasWorkflowExecutionCompletedEventAttributes())
              .getWorkflowExecutionCompletedEventAttributes();
      assertEquals(expected, SerContext.firstSignature(completed.getResult()));

      var signaled =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionSignaled",
                  e -> e.hasWorkflowExecutionSignaledEventAttributes())
              .getWorkflowExecutionSignaledEventAttributes();
      assertEquals(expected, SerContext.firstSignature(signaled.getInput()));

      var accepted =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionUpdateAccepted",
                  e -> e.hasWorkflowExecutionUpdateAcceptedEventAttributes())
              .getWorkflowExecutionUpdateAcceptedEventAttributes();
      assertEquals(
          expected, SerContext.firstSignature(accepted.getAcceptedRequest().getInput().getArgs()));

      var updateCompleted =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionUpdateCompleted",
                  e -> e.hasWorkflowExecutionUpdateCompletedEventAttributes())
              .getWorkflowExecutionUpdateCompletedEventAttributes();
      assertEquals(
          expected, SerContext.firstSignature(updateCompleted.getOutcome().getSuccess()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
