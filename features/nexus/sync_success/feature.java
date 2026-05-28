package nexus.sync_success;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
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

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service
  interface TestService {
    @Operation
    String syncOperation(String name);
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
      return stub.syncOperation("world");
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
      assertEquals("Hello, world!", result);
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      // Assert that the sync operation transitioned straight from Scheduled to
      // Completed with no Started event.
      var events = runner.getWorkflowHistory(run).getEventsList();
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationScheduledEventAttributes()),
          "expected NexusOperationScheduled event in history");
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationCompletedEventAttributes()),
          "expected NexusOperationCompleted event in history");
      assertFalse(
          events.stream().anyMatch(e -> e.hasNexusOperationStartedEventAttributes()),
          "unexpected NexusOperationStarted event for sync operation");
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> syncOperation() {
      return OperationHandler.sync((ctx, details, name) -> "Hello, " + name + "!");
    }
  }
}
