package schedule.trigger;

import io.temporal.client.WorkflowOptions;
import io.temporal.client.schedules.*;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.Arrays;
import java.util.UUID;
import org.junit.jupiter.api.Assertions;

@WorkflowInterface
public interface feature extends Feature {

  @WorkflowMethod
  String workflow(String arg);

  class Impl implements feature {
    @Override
    public String workflow(String arg) {
      return arg;
    }

    @Override
    public Run execute(Runner runner) {
      ScheduleClientOptions option =
          ScheduleClientOptions.newBuilder().setNamespace(runner.config.namespace).build();
      ScheduleClient client = ScheduleClient.newInstance(runner.service, option);

      String workflowId = UUID.randomUUID().toString();
      String scheduleId = UUID.randomUUID().toString();

      ScheduleHandle handle =
          client.createSchedule(
              scheduleId,
              Schedule.newBuilder()
                  .setAction(
                      ScheduleActionStartWorkflow.newBuilder()
                          .setWorkflowType(feature.class)
                          .setOptions(
                              WorkflowOptions.newBuilder()
                                  .setWorkflowId(workflowId)
                                  .setTaskQueue(runner.config.taskQueue)
                                  .build())
                          .setArguments("arg1")
                          .build())
                  .setSpec(ScheduleSpec.newBuilder().setIntervals(Arrays.asList()).build())
                  .setState(ScheduleState.newBuilder().setPaused(true).build())
                  .build(),
              ScheduleOptions.newBuilder().build());

      try {
        handle.trigger();
        // We have to wait before triggering again. See
        // https://github.com/temporalio/temporal/issues/3614
        Thread.sleep(2_000);
        handle.trigger();
        runner.retry(
            () -> handle.describe().getInfo().getNumActions() == 2, 3, Duration.ofSeconds(1));
      } catch (Exception e) {
        Assertions.fail();
      } finally {
        handle.delete();
      }
      return null;
    }
  }
}
