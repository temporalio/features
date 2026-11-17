package nexus.schedule_to_start_timeout;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.temporal.api.enums.v1.EventType;
import io.temporal.api.enums.v1.TimeoutType;
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.NexusOperationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.NexusOperationOptions;
import io.temporal.workflow.NexusServiceOptions;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "unhandled")
    String unhandled(String name);
  }

  class Impl implements feature {
    @Override
    public String workflow(String endpoint) {
      // No Nexus service is registered: nothing can handle this operation.
      var options =
          NexusServiceOptions.newBuilder()
              .setEndpoint(endpoint)
              .setOperationOptions(
                  NexusOperationOptions.newBuilder()
                      .setScheduleToCloseTimeout(Duration.ofSeconds(30))
                      .setScheduleToStartTimeout(Duration.ofSeconds(3))
                      .build())
              .build();
      var stub = Workflow.newNexusServiceStub(TestService.class, options);
      var handle = Workflow.startNexusOperation(stub::unhandled, "world");
      var executionFailure =
          assertThrows(NexusOperationFailure.class, () -> handle.getExecution().get());
      var resultFailure = assertThrows(NexusOperationFailure.class, () -> handle.getResult().get());
      for (var failure : new NexusOperationFailure[] {executionFailure, resultFailure}) {
        var operationFailure =
            assertOperationFailure(failure, endpoint, "test-service", "unhandled", "");
        assertTimeoutCause(operationFailure.getCause(), TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_START);
      }
      return "timed out as expected";
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(
          WorkflowOptions.newBuilder()
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .build(),
          runner.nexusEndpoint);
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      assertEquals("timed out as expected", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT);
    }
  }
}
