package schedule.basic;

import io.temporal.api.enums.v1.ScheduleOverlapPolicy;
import io.temporal.client.WorkflowOptions;
import io.temporal.client.schedules.*;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.Arrays;
import java.util.Optional;
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
                              Arrays.asList(new ScheduleIntervalSpec(Duration.ofSeconds(2))))
                          .build())
                  .setPolicy(
                      SchedulePolicy.newBuilder()
                          .setOverlap(ScheduleOverlapPolicy.SCHEDULE_OVERLAP_POLICY_BUFFER_ONE)
                          .build())
                  .build(),
              ScheduleOptions.newBuilder().build());

      try {
        // Confirm simple describe
        ScheduleDescription description = handle.describe();
        Assertions.assertEquals(
            workflowId,
            ((ScheduleActionStartWorkflow) description.getSchedule().getAction())
                .getOptions()
                .getWorkflowId());
        // Confirm simple list
        Thread.sleep(10000);
        runner.retry(
            () ->
                client.listSchedules().filter(s -> s.getScheduleId().equals(scheduleId)).count()
                    == 1,
            10,
            Duration.ofSeconds(1));
        // Wait for first completion
        runner.retry(
            () -> isWorkflowCompletedWith(runner, workflowId, "arg1"), 10, Duration.ofSeconds(1));
        // Update and change arg
        handle.update(
            (ScheduleUpdateInput input) -> {
              Schedule.Builder builder = Schedule.newBuilder(input.getDescription().getSchedule());
              ScheduleActionStartWorkflow wfAction =
                  ((ScheduleActionStartWorkflow) input.getDescription().getSchedule().getAction());
              builder.setAction(
                  ScheduleActionStartWorkflow.newBuilder(wfAction).setArguments("arg2").build());
              return new ScheduleUpdate(builder.build());
            });
        // Wait for next completion
        runner.retry(
            () -> isWorkflowCompletedWith(runner, workflowId, "arg2"), 10, Duration.ofSeconds(1));
      } catch (Exception e) {
        Assertions.fail();
      } finally {
        handle.delete();
      }
      return null;
    }
  }

  private static Boolean isWorkflowCompletedWith(Runner runner, String workflowId, String result) {
    return runner
            .client
            .listExecutions("WorkflowType = 'feature'")
            .filter(
                wm ->
                    wm.getWorkflowExecutionInfo()
                        .getExecution()
                        .getWorkflowId()
                        .startsWith(workflowId))
            .filter(
                wm ->
                    wm.getStatus()
                        .equals(
                            io.temporal.api.enums.v1.WorkflowExecutionStatus
                                .WORKFLOW_EXECUTION_STATUS_COMPLETED))
            .filter(
                wm ->
                    runner
                        .client
                        .newUntypedWorkflowStub(
                            wm.getWorkflowExecutionInfo().getExecution(), Optional.empty())
                        .getResult(String.class)
                        .equals(result))
            .count()
        > 0;
  }
}
