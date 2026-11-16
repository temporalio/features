package nexus.sync_operation_canceled;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.failure.CanceledFailure;
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
    @Operation(name = "cancel")
    String canceledOperation(String name);
  }

  class Impl implements feature {
    @Override
    public String workflow(String endpoint) {
      var options =
          NexusServiceOptions.newBuilder()
              .setEndpoint(endpoint)
              .setOperationOptions(
                  NexusOperationOptions.newBuilder()
                      .setScheduleToCloseTimeout(Duration.ofMinutes(1))
                      .build())
              .build();
      var stub = Workflow.newNexusServiceStub(TestService.class, options);
      var failure = assertThrows(RuntimeException.class, () -> stub.canceledOperation("world"));
      var operationFailure =
          assertOperationFailure(failure, endpoint, "test-service", "cancel", "");
      assertInstanceOf(CanceledFailure.class, operationFailure.getCause());
      var canceledFailure = (CanceledFailure) operationFailure.getCause();
      assertEquals("operation canceled by handler", canceledFailure.getOriginalMessage());
      assertEquals(0, canceledFailure.getDetails().getSize());
      assertApplicationCause(
          canceledFailure.getCause(),
          "CancellationCause",
          "cancellation cause",
          "cancellation detail");
      return "canceled by handler";
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
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
      assertEquals("canceled by handler", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      // No caller cancellation request and no async started event.
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_CANCELED);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> canceledOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            throw OperationException.canceled(
                "operation canceled by handler",
                ApplicationFailure.newFailure(
                    "cancellation cause", "CancellationCause", "cancellation detail"));
          });
    }
  }
}
