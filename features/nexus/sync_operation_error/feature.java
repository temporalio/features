package nexus.sync_operation_error;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;

import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
import io.nexusrpc.Service;
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

@WorkflowInterface
public interface feature extends Feature {
  String ERROR_TYPE = "TestFailure";
  String ERROR_MESSAGE = "deliberate failure";
  String ERROR_DETAIL = "application detail";

  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "fail")
    String failingOperation(String name);
  }

  class Impl implements feature {
    @Override
    public String workflow(String endpoint) {
      var serviceOptions =
          NexusServiceOptions.newBuilder()
              .setEndpoint(endpoint)
              .setOperationOptions(
                  NexusOperationOptions.newBuilder()
                      .setScheduleToCloseTimeout(Duration.ofMinutes(1))
                      .build())
              .build();
      TestService stub = Workflow.newNexusServiceStub(TestService.class, serviceOptions);
      try {
        stub.failingOperation("world");
        return "no error";
      } catch (NexusOperationFailure e) {
        assertOperationFailure(e, endpoint, "test-service", "fail", "");
        assertInstanceOf(ApplicationFailure.class, e.getCause());
        var wrapper = (ApplicationFailure) e.getCause();
        assertEquals("OperationError", wrapper.getType());
        assertEquals("operation failed", wrapper.getOriginalMessage());
        assertApplicationCause(wrapper.getCause(), ERROR_TYPE, ERROR_MESSAGE, ERROR_DETAIL);
        return ERROR_TYPE + ": " + ERROR_MESSAGE;
      }
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var options =
          WorkflowOptions.newBuilder()
              .setTaskQueue(runner.config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
              .build();
      return runner.executeSingleWorkflow(options, runner.nexusEndpoint);
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var result = runner.waitForRunResult(run, String.class);
      assertEquals(ERROR_TYPE + ": " + ERROR_MESSAGE, result);
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
    public OperationHandler<String, String> failingOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            throw OperationException.failed(
                "operation failed",
                ApplicationFailure.newFailure(ERROR_MESSAGE, ERROR_TYPE, ERROR_DETAIL));
          });
    }
  }
}
