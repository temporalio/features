package schedule.pause;

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
                  .setSpec(
                      ScheduleSpec.newBuilder()
                          .setIntervals(
                              Arrays.asList(new ScheduleIntervalSpec(Duration.ofMinutes(1))))
                          .build())
                  .setState(
                      ScheduleState.newBuilder().setPaused(true).setNote("initial note").build())
                  .build(),
              ScheduleOptions.newBuilder().build());

      try {
        // Confirm pause
        ScheduleState state = handle.describe().getSchedule().getState();
        Assertions.assertEquals(true, state.isPaused());
        Assertions.assertEquals("initial note", state.getNote());
        // Re-pause
        handle.pause("custom note1");
        state = handle.describe().getSchedule().getState();
        Assertions.assertEquals(true, state.isPaused());
        Assertions.assertEquals("custom note1", state.getNote());
        // Unpause
        handle.unpause();
        state = handle.describe().getSchedule().getState();
        Assertions.assertEquals(false, state.isPaused());
        Assertions.assertEquals("Unpaused via Java SDK", state.getNote());
        // Re-unpause
        handle.unpause("custom note2");
        state = handle.describe().getSchedule().getState();
        Assertions.assertEquals(false, state.isPaused());
        Assertions.assertEquals("custom note2", state.getNote());
        // Pause
        handle.pause();
        state = handle.describe().getSchedule().getState();
        Assertions.assertEquals(true, state.isPaused());
        Assertions.assertEquals("Paused via Java SDK", state.getNote());
      } catch (Exception e) {
        Assertions.fail();
      } finally {
        handle.delete();
      }
      return null;
    }
  }
}
