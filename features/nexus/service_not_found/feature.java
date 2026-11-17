package nexus.service_not_found;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.HandlerException;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
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
    @Operation(name = "echo")
    String echo(String input);
  }

  @Service(name = "missing-service")
  interface MissingService {
    @Operation(name = "echo")
    String echo(String input);
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
      var stub = Workflow.newNexusServiceStub(MissingService.class, options);
      var failure = assertThrows(NexusOperationFailure.class, () -> stub.echo("world"));
      var operationFailure =
          assertOperationFailure(failure, endpoint, "missing-service", "echo", "");
      assertInstanceOf(HandlerException.class, operationFailure.getCause());
      var handlerFailure = (HandlerException) operationFailure.getCause();
      assertEquals(HandlerException.ErrorType.NOT_FOUND, handlerFailure.getErrorType());
      assertEquals(HandlerException.RetryBehavior.UNSPECIFIED, handlerFailure.getRetryBehavior());
      assertFalse(handlerFailure.isRetryable());
      assertTrue(handlerFailure.getMessage().contains("missing-service"));
      return "service not found";
    }

    @Override
    public Object[] nexusServiceImplementations() {
      // A real service must be registered so this is a lookup failure on a
      // polling worker, not a task waiting indefinitely for a worker.
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
      assertEquals("service not found", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_FAILED);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> echo() {
      return OperationHandler.sync((context, details, input) -> input);
    }
  }
}
