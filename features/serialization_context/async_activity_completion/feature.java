package serialization_context.async_activity_completion;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

import io.temporal.activity.Activity;
import io.temporal.activity.ActivityInfo;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.payload.context.ActivitySerializationContext;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.concurrent.atomic.AtomicReference;
import serialization_context.sercontext.SerContext;

@WorkflowInterface
public interface feature extends Feature {

  String ACTIVITY_RESULT = "completed-out-of-band";
  String HEARTBEAT_DATA = "beat";

  /** What the activity worker saw, used by the completing client to rebuild the same context. */
  AtomicReference<ActivityInfo> SCHEDULED = new AtomicReference<>();

  @WorkflowMethod
  String workflow();

  @ActivityInterface
  interface Activities {
    @ActivityMethod
    String pendingActivity();

    class Impl implements Activities {
      @Override
      public String pendingActivity() {
        var context = Activity.getExecutionContext();
        SCHEDULED.set(context.getInfo());
        context.doNotCompleteOnReturn();
        return null;
      }
    }
  }

  class Impl implements feature {

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerActivitiesImplementations(new Activities.Impl());
    }

    @Override
    public String workflow() {
      var activities =
          Workflow.newActivityStub(
              Activities.class,
              ActivityOptions.newBuilder()
                  .setStartToCloseTimeout(Duration.ofMinutes(1))
                  .setHeartbeatTimeout(Duration.ofSeconds(30))
                  .build());
      return activities.pendingActivity();
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(SerContext.dataConverter());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var run = runner.executeSingleParameterlessWorkflow();

      ActivityInfo info = null;
      for (int i = 0; i < 300 && info == null; i++) {
        info = SCHEDULED.get();
        if (info == null) {
          Thread.sleep(100);
        }
      }
      assertNotNull(info, "activity was never started");

      // A task token carries no activity metadata, so the context has to be supplied.
      var completionClient =
          runner
              .client
              .newActivityCompletionClient()
              .withContext(new ActivitySerializationContext(info));
      completionClient.heartbeat(info.getTaskToken(), HEARTBEAT_DATA);
      completionClient.complete(info.getTaskToken(), ACTIVITY_RESULT);
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(ACTIVITY_RESULT, runner.waitForRunResult(run, String.class));

      var info = SCHEDULED.get();
      var completed =
          SerContext.findEvent(
                  runner.getWorkflowHistory(run),
                  "ActivityTaskCompleted",
                  e -> e.hasActivityTaskCompletedEventAttributes())
              .getActivityTaskCompletedEventAttributes();
      assertEquals(
          SerContext.activitySignature(
              info.getNamespace(),
              info.getWorkflowId(),
              info.getWorkflowType(),
              info.getActivityType(),
              info.getActivityTaskQueue(),
              false),
          SerContext.firstSignature(completed.getResult()));
    }

    @Override
    public void checkHistory(Runner runner, Run run) {
      // The replayer runs histories under a placeholder namespace and workflow ID, which a context
      // derived signature can never match.
    }
  }
}
