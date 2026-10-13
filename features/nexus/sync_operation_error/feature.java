package nexus.sync_operation_error;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
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
  String ERROR_TYPE = "TestFailure";
  String ERROR_MESSAGE = "deliberate failure";

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
        var applicationFailure = findApplicationFailure(e, ERROR_TYPE);
        if (applicationFailure == null) {
          throw ApplicationFailure.newNonRetryableFailure(
              "expected an application failure of type "
                  + ERROR_TYPE
                  + " in the cause chain, got "
                  + e,
              "AssertionFailure");
        }
        return applicationFailure.getType() + ": " + applicationFailure.getOriginalMessage();
      }
    }

    private static ApplicationFailure findApplicationFailure(Throwable failure, String type) {
      for (Throwable cause = failure; cause != null; cause = cause.getCause()) {
        if (cause instanceof ApplicationFailure
            && type.equals(((ApplicationFailure) cause).getType())) {
          return (ApplicationFailure) cause;
        }
      }
      return null;
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
      var events = runner.getWorkflowHistory(run).getEventsList();
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationFailedEventAttributes()),
          "expected NexusOperationFailed event in history");
      assertFalse(
          events.stream().anyMatch(e -> e.hasNexusOperationCompletedEventAttributes()),
          "unexpected NexusOperationCompleted event in history");
      runner.checkCurrentAndPastHistories(run);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> failingOperation() {
      return OperationHandler.sync(
          (context, details, name) -> {
            throw OperationException.failure(
                ApplicationFailure.newFailure(ERROR_MESSAGE, ERROR_TYPE));
          });
    }
  }
}
