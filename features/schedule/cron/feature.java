package schedule.cron;

import com.google.common.base.Preconditions;
import io.temporal.api.enums.v1.WorkflowExecutionStatus;
import io.temporal.api.workflowservice.v1.ListWorkflowExecutionsRequest;
import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.*;
import io.temporal.workflow.Workflow;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public interface feature extends Feature, SimpleWorkflow {
  Logger log = LoggerFactory.getLogger(feature.class);

  class Impl implements feature {
    @Override
    public void workflow() {
      Preconditions.checkState("@every 2s".equals(Workflow.getInfo().getCronSchedule()));
    }
  }

  @Override
  default void workflowOptions(WorkflowOptions.Builder builder) {
    builder.setCronSchedule("@every 2s");
  }

  @Override
  default void checkResult(Runner runner, Run run) throws Exception {
    try {
      // Try 10 times (sleeping 1s before each) to get at least two complete executions
      for (var i = 0; i < 10; i++) {
        Thread.sleep(1000);
        var resp = runner.client.getWorkflowServiceStubs().blockingStub().
                listWorkflowExecutions(ListWorkflowExecutionsRequest.newBuilder()
                        .setNamespace(runner.config.namespace)
                        .setQuery("WorkflowId = '" + run.execution.getWorkflowId() + "'")
                        .build());
        var completed = resp.getExecutionsList().stream().filter(exec -> {
          if (exec.getStatus() == WorkflowExecutionStatus.WORKFLOW_EXECUTION_STATUS_COMPLETED) {
            return true;
          }
          Assertions.assertEquals(exec.getStatus(), WorkflowExecutionStatus.WORKFLOW_EXECUTION_STATUS_RUNNING);
          return false;
        }).count();
        if (completed >= 2) {
          return;
        }
      }
      throw new RuntimeException("Did not get at least 2 completed");
    } finally {
      // Terminate workflow
      try {
        runner.client.newUntypedWorkflowStub(run.execution.getWorkflowId()).terminate("feature complete");
      } catch (Exception e) {
        log.warn("Failed terminating workflow", e);
      }
    }
  }
}