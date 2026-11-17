package nexus.async_cancellation_abandon;

import static io.temporal.sdkfeatures.NexusAssertions.assertOperationFailure;
import static org.junit.jupiter.api.Assertions.*;

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
import io.temporal.workflow.CancellationScope;
import io.temporal.workflow.CompletablePromise;
import io.temporal.workflow.NexusOperationCancellationType;
import io.temporal.workflow.NexusOperationOptions;
import io.temporal.workflow.NexusServiceOptions;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.time.Duration;
import java.util.List;
import java.util.stream.Collectors;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "test-service")
  interface TestService {
    @Operation(name = "block")
    String block(String id);

    @Operation(name = "release")
    String release(String id);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String run(String id);

    @SignalMethod
    void release();
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    private boolean released;

    @Override
    public String run(String id) {
      Workflow.await(() -> released);
      CancellationScope.throwCanceled();
      return "released";
    }

    @Override
    public void release() {
      released = true;
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
                      .setCancellationType(NexusOperationCancellationType.ABANDON)
                      .build())
              .build();
      var stub = Workflow.newNexusServiceStub(TestService.class, options);
      String id = Workflow.getInfo().getWorkflowId() + "-handler";
      CompletablePromise<String> outcome = Workflow.newPromise();
      var scope =
          Workflow.newCancellationScope(
              () -> {
                var handle = Workflow.startNexusOperation(stub::block, id);
                assertFalse(handle.getExecution().get().getOperationToken().orElse("").isEmpty());
                outcome.completeFrom(
                    handle
                        .getResult()
                        .handle(
                            (value, failure) -> {
                              var operationFailure =
                                  assertOperationFailure(
                                      failure, endpoint, "test-service", "block", "");
                              assertInstanceOf(CanceledFailure.class, operationFailure.getCause());
                              var canceled = (CanceledFailure) operationFailure.getCause();
                              assertNull(canceled.getCause());
                              assertEquals(0, canceled.getDetails().getSize());
                              return "canceled";
                            }));
              });
      scope.run();
      scope.cancel();
      assertEquals("canceled", outcome.get());
      // The operation must remain live after local cancellation. Release it
      // through Nexus so only the service accesses the backing workflow.
      return stub.release(id);
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
      assertEquals("released", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      var events = runner.getWorkflowHistory(run).getEventsList();
      var scheduled =
          events.stream()
              .filter(e -> e.hasNexusOperationScheduledEventAttributes())
              .collect(Collectors.toList());
      assertEquals(
          List.of("block", "release"),
          scheduled.stream()
              .map(e -> e.getNexusOperationScheduledEventAttributes().getOperation())
              .collect(Collectors.toList()));
      long releaseId = scheduled.get(1).getEventId();
      assertEquals(
          1, events.stream().filter(e -> e.hasNexusOperationStartedEventAttributes()).count());
      assertTrue(
          events.stream()
              .anyMatch(
                  e ->
                      e.hasNexusOperationCompletedEventAttributes()
                          && e.getNexusOperationCompletedEventAttributes().getScheduledEventId()
                              == releaseId));
      for (var event : events) {
        assertFalse(event.hasNexusOperationCancelRequestedEventAttributes());
        assertFalse(event.hasNexusOperationCancelRequestCompletedEventAttributes());
        assertFalse(event.hasNexusOperationCancelRequestFailedEventAttributes());
        assertFalse(event.hasNexusOperationCanceledEventAttributes());
        assertFalse(event.hasNexusOperationFailedEventAttributes());
        assertFalse(event.hasNexusOperationTimedOutEventAttributes());
        if (event.hasNexusOperationCompletedEventAttributes()) {
          assertTrue(event.getEventId() > releaseId);
        }
      }
      runner.checkCurrentAndPastHistories(run);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> block() {
      return WorkflowRunOperation.fromWorkflowMethod(
          (context, details, id) ->
              Nexus.getOperationContext()
                      .getWorkflowClient()
                      .newWorkflowStub(
                          HandlerWorkflow.class,
                          WorkflowOptions.newBuilder().setWorkflowId(id).build())
                  ::run);
    }

    @OperationImpl
    public OperationHandler<String, String> release() {
      return OperationHandler.sync(
          (context, details, id) -> {
            var stub = Nexus.getOperationContext().getWorkflowClient().newUntypedWorkflowStub(id);
            stub.signal("release");
            return stub.getResult(String.class);
          });
    }
  }
}
