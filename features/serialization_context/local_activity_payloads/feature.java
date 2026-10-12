package serialization_context.local_activity_payloads;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.activity.LocalActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String WORKFLOW_INPUT = "hello";
  String ACTIVITY_TYPE = "SerCtxLocalActivity";

  @WorkflowMethod
  String workflow(String input);

  @ActivityInterface
  interface Activities {
    @ActivityMethod(name = ACTIVITY_TYPE)
    String localActivity(String input);

    class Impl implements Activities {
      @Override
      public String localActivity(String input) {
        return input + "|local";
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerActivitiesImplementations(new Activities.Impl());
    }

    @Override
    public String workflow(String input) {
      var activities =
          Workflow.newLocalActivityStub(
              Activities.class,
              LocalActivityOptions.newBuilder()
                  .setStartToCloseTimeout(Duration.ofSeconds(10))
                  .build());
      return activities.localActivity(input);
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
      assertEquals(WORKFLOW_INPUT + "|local", runner.waitForRunResult(run, String.class));

      var history = runner.getWorkflowHistory(run);
      var started =
          SerContext.findEvent(
                  history,
                  "WorkflowExecutionStarted",
                  e -> e.hasWorkflowExecutionStartedEventAttributes())
              .getWorkflowExecutionStartedEventAttributes();

      // Local activity payloads never reach history, so the context is asserted on what the codec
      // was actually asked to convert with.
      var expected =
          SerContext.activitySignature(
              runner.config.namespace,
              run.execution.getWorkflowId(),
              started.getWorkflowType().getName(),
              ACTIVITY_TYPE,
              started.getTaskQueue().getName(),
              true);
      assertTrue(
          SerContext.OBSERVED_SIGNATURES.contains(expected),
          "no local activity context observed, wanted "
              + expected
              + ", got "
              + SerContext.OBSERVED_SIGNATURES);
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
