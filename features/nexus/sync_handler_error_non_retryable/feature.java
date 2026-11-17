package nexus.sync_handler_error_non_retryable;

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
import io.temporal.failure.ApplicationFailure;
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
import java.util.concurrent.atomic.AtomicInteger;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "fail")
    String failingOperation(String name);
  }

  class Impl implements feature {
    private final TestServiceImpl service = new TestServiceImpl();

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
      var failure = assertThrows(NexusOperationFailure.class, () -> stub.failingOperation("world"));
      var operationFailure = assertOperationFailure(failure, endpoint, "test-service", "fail", "");
      assertInstanceOf(HandlerException.class, operationFailure.getCause());
      var handlerFailure = (HandlerException) operationFailure.getCause();
      assertEquals(HandlerException.ErrorType.UNAVAILABLE, handlerFailure.getErrorType());
      assertEquals(HandlerException.RetryBehavior.NON_RETRYABLE, handlerFailure.getRetryBehavior());
      assertFalse(handlerFailure.isRetryable());
      assertEquals("handler rejected request", handlerFailure.getMessage());
      assertApplicationCause(
          handlerFailure.getCause(), "HandlerCause", "handler cause", "handler detail");
      return "handler failed as expected";
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {service};
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      service.attempts.set(0);
      return runner.executeSingleWorkflow(
          WorkflowOptions.newBuilder()
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .build(),
          runner.nexusEndpoint);
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      assertEquals("handler failed as expected", runner.waitForRunResult(run, String.class));
      assertEquals(
          1, service.attempts.get(), "expected no retries of the non-retryable handler error");
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
    // Count handler calls only; workflow execution and replay never read this state.
    final AtomicInteger attempts = new AtomicInteger();

    @OperationImpl
    public OperationHandler<String, String> failingOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            attempts.incrementAndGet();
            // UNAVAILABLE normally retries: the explicit override must win.
            throw new HandlerException(
                HandlerException.ErrorType.UNAVAILABLE,
                "handler rejected request",
                ApplicationFailure.newFailure("handler cause", "HandlerCause", "handler detail"),
                HandlerException.RetryBehavior.NON_RETRYABLE);
          });
    }
  }
}
