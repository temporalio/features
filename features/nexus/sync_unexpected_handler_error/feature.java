package nexus.sync_unexpected_handler_error;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.assertEquals;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowOptions;
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
    @Operation(name = "retry")
    String retryingOperation(String name);
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
      return Workflow.newNexusServiceStub(TestService.class, options).retryingOperation("world");
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
      assertEquals("recovered", runner.waitForRunResult(run, String.class));
      assertEquals(
          2, service.attempts.get(), "expected one failed handler call and one successful retry");
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_COMPLETED);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    // Count handler calls only; workflow execution and replay never read this state.
    final AtomicInteger attempts = new AtomicInteger();

    @OperationImpl
    public OperationHandler<String, String> retryingOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            if (attempts.incrementAndGet() == 1) {
              // Deliberately neither a Nexus handler exception nor an operation exception.
              throw new IllegalStateException("unexpected implementation failure");
            }
            return "recovered";
          });
    }
  }
}
