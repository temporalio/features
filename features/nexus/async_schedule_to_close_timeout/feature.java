package nexus.async_schedule_to_close_timeout;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.api.enums.v1.TimeoutType;
import io.temporal.client.WorkflowOptions;
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
  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "block-forever")
    String blockingOperation(String id);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String handlerWorkflow(String id);
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    @Override
    public String handlerWorkflow(String id) {
      Workflow.await(() -> false);
      return "unreachable";
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
                      .setScheduleToCloseTimeout(Duration.ofSeconds(10))
                      .build())
              .build();
      var stub = Workflow.newNexusServiceStub(TestService.class, options);
      var handle =
          Workflow.startNexusOperation(
              stub::blockingOperation, Workflow.getInfo().getWorkflowId() + "-handler");
      var token = handle.getExecution().get().getOperationToken().orElse("");
      assertFalse(token.isEmpty());
      var failure = assertThrows(NexusOperationFailure.class, () -> handle.getResult().get());
      var operationFailure =
          assertOperationFailure(failure, endpoint, "test-service", "block-forever", token);
      assertTimeoutCause(operationFailure.getCause(), TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_CLOSE);
      return "timed out as expected";
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
      try {
        assertEquals("timed out as expected", runner.waitForRunResult(run, String.class));
      } finally {
        // A Nexus timeout does not cancel the backing workflow.
        runner.client.newUntypedWorkflowStub(run.execution.getWorkflowId() + "-handler").cancel();
      }
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_STARTED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_TIMED_OUT);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> blockingOperation() {
      return WorkflowRunOperation.fromWorkflowMethod(
          (context, details, id) ->
              Nexus.getOperationContext()
                      .getWorkflowClient()
                      .newWorkflowStub(
                          HandlerWorkflow.class,
                          WorkflowOptions.newBuilder()
                              .setWorkflowId(id)
                              .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
                              .build())
                  ::handlerWorkflow);
    }
  }
}
