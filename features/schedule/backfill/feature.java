package schedule.backfill;

import io.temporal.api.enums.v1.ScheduleOverlapPolicy;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.schedules.*;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.time.Instant;
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
                  .setState(ScheduleState.newBuilder().setPaused(true).build())
                  .build(),
              ScheduleOptions.newBuilder().build());

      try {
        // Run backfill
        Instant now = Instant.now();
        Instant threeYearsAgo = now.minus(Duration.ofDays(3 * 365));
        Instant thirtyMinutesAgo = now.minus(Duration.ofMinutes(30));
        handle.backfill(
            Arrays.asList(
                new ScheduleBackfill(
                    threeYearsAgo.minus(Duration.ofMinutes(2)),
                    threeYearsAgo,
                    ScheduleOverlapPolicy.SCHEDULE_OVERLAP_POLICY_ALLOW_ALL),
                new ScheduleBackfill(
                    thirtyMinutesAgo.minus(Duration.ofMinutes(2)),
                    thirtyMinutesAgo,
                    ScheduleOverlapPolicy.SCHEDULE_OVERLAP_POLICY_ALLOW_ALL)));
        // Confirm 4 executions
        runner.retry(
            () -> handle.describe().getInfo().getNumActions() == 4, 5, Duration.ofSeconds(1));
      } catch (Exception e) {
        Assertions.fail();
      } finally {
        handle.delete();
      }
      return null;
    }
  }
}
