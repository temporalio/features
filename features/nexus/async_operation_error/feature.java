package nexus.async_operation_error;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.failure.NexusOperationFailure;
import io.temporal.nexus.Nexus;
import io.temporal.nexus.WorkflowRunOperation;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.NexusOperationOptions;
import io.temporal.workflow.NexusServiceOptions;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;

@WorkflowInterface
public interface feature extends Feature {
  String ERROR_TYPE = "TestFailure";
  String ERROR_MESSAGE = "deliberate workflow failure";
  String ERROR_DETAIL = "application detail";

  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "fail-async")
    String failingOperation(String name);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String handlerWorkflow(String name);
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    @Override
    public String handlerWorkflow(String name) {
      throw ApplicationFailure.newFailure(ERROR_MESSAGE, ERROR_TYPE, ERROR_DETAIL);
    }
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
      var handle = Workflow.startNexusOperation(stub::failingOperation, "world");
      var token = handle.getExecution().get().getOperationToken().orElse("");
      assertFalse(token.isEmpty());
      var failure = assertThrows(NexusOperationFailure.class, () -> handle.getResult().get());
      var operationFailure =
          assertOperationFailure(failure, endpoint, "test-service", "fail-async", token);
      // No workflow-client or sync OperationError wrapper should be inserted.
      assertApplicationCause(operationFailure.getCause(), ERROR_TYPE, ERROR_MESSAGE, ERROR_DETAIL);
      return "failed as expected";
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
    }

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(HandlerWorkflowImpl.class);
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
      assertEquals("failed as expected", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_STARTED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_FAILED);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> failingOperation() {
      return WorkflowRunOperation.fromWorkflowMethod(
          (context, details, name) ->
              Nexus.getOperationContext()
                      .getWorkflowClient()
                      .newWorkflowStub(
                          HandlerWorkflow.class,
                          WorkflowOptions.newBuilder()
                              .setWorkflowId(details.getRequestId())
                              .build())
                  ::handlerWorkflow);
    }
  }
}
