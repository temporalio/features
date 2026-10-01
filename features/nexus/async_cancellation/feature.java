package nexus.async_cancellation;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.CanceledFailure;
import io.temporal.nexus.Nexus;
import io.temporal.nexus.WorkflowRunOperation;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.Worker;
import io.temporal.workflow.CompletablePromise;
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
    String blockingOperation(String name);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String handlerWorkflow(String name);
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    @Override
    public String handlerWorkflow(String name) {
      Workflow.await(() -> false);
      return "unreachable";
    }
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

      CompletablePromise<String> outcome = Workflow.newPromise();
      var scope =
          Workflow.newCancellationScope(
              () -> {
                var handle = Workflow.startNexusOperation(stub::blockingOperation, "world");
                handle.getExecution().get();
                handle
                    .getResult()
                    .handle(
                        (value, failure) -> {
                          outcome.complete(describeOutcome(failure));
                          return null;
                        });
              });
      scope.run();
      scope.cancel();
      return outcome.get();
    }

    private static String describeOutcome(RuntimeException failure) {
      if (failure == null) {
        return "completed";
      }
      for (Throwable cause = failure; cause != null; cause = cause.getCause()) {
        if (cause instanceof CanceledFailure) {
          return "canceled";
        }
      }
      return "failed with " + failure;
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
      assertEquals("canceled", result);
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      var events = runner.getWorkflowHistory(run).getEventsList();
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationCancelRequestedEventAttributes()),
          "expected NexusOperationCancelRequested event in history");
      assertTrue(
          events.stream().anyMatch(e -> e.hasNexusOperationCanceledEventAttributes()),
          "expected NexusOperationCanceled event in history");
      runner.checkCurrentAndPastHistories(run);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> blockingOperation() {
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
