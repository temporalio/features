package schedule.duplicate_error;

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
  void workflow();

  class Impl implements feature {
    @Override
    public void workflow() {}

    @Override
    public Run execute(Runner runner) {
      ScheduleClientOptions option =
          ScheduleClientOptions.newBuilder().setNamespace(runner.config.namespace).build();
      ScheduleClient client = ScheduleClient.newInstance(runner.service, option);

      String scheduleId = UUID.randomUUID().toString();

      Schedule schedule =
          Schedule.newBuilder()
              .setAction(
                  ScheduleActionStartWorkflow.newBuilder()
                      .setWorkflowType(feature.class)
                      .setOptions(
                          WorkflowOptions.newBuilder()
                              .setWorkflowId(UUID.randomUUID().toString())
                              .setTaskQueue(runner.config.taskQueue)
                              .build())
                      .build())
              .setSpec(
                  ScheduleSpec.newBuilder()
                      .setIntervals(Arrays.asList(new ScheduleIntervalSpec(Duration.ofHours(1))))
                      .build())
              .setState(ScheduleState.newBuilder().setPaused(true).build())
              .build();

      ScheduleHandle handle =
          client.createSchedule(scheduleId, schedule, ScheduleOptions.newBuilder().build());

      try {
        // Creating again with the same schedule ID should throw ScheduleAlreadyRunningException.
        Assertions.assertThrows(
            ScheduleAlreadyRunningException.class,
            () ->
                client.createSchedule(scheduleId, schedule, ScheduleOptions.newBuilder().build()));
      } finally {
        handle.delete();
      }
      return null;
    }
  }
}
