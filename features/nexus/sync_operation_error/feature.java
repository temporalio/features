package nexus.sync_operation_error;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
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
  @WorkflowMethod
  String workflow(String endpoint);

  @Service
  interface TestService {
    @Operation
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
        Throwable cause = e.getCause();
        while (cause != null && !(cause instanceof ApplicationFailure)) {
          cause = cause.getCause();
        }
        var applicationFailure = (ApplicationFailure) cause;
        return "caught "
            + applicationFailure.getType()
            + ": "
            + applicationFailure.getOriginalMessage();
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
      assertEquals("caught TestError: deliberate failure", result);
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      var events = runner.getWorkflowHistory(run).getEventsList();
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationFailedEventAttributes()),
          "expected NexusOperationFailed event in history");
      assertFalse(
          events.stream().anyMatch(e -> e.hasNexusOperationCompletedEventAttributes()),
          "unexpected NexusOperationCompleted event in history");
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> failingOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            throw ApplicationFailure.newNonRetryableFailure("deliberate failure", "TestError");
          });
    }
  }
}
