package nexus.async_cancellation_wait_requested;

import static io.temporal.sdkfeatures.NexusAssertions.assertOperationFailure;
import static org.junit.jupiter.api.Assertions.*;

import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationCancelDetails;
import io.nexusrpc.handler.OperationContext;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.OperationStartDetails;
import io.nexusrpc.handler.OperationStartResult;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityOptions;
import io.temporal.client.WorkflowFailedException;
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
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
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

  @ActivityInterface
  interface GateActivities {
    void checkpoint(boolean release);
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
      // Delivery must not finish the workflow until the caller regains control.
      Workflow.newDetachedCancellationScope(() -> Workflow.await(() -> released)).run();
      if (!CancellationScope.current().isCancelRequested()) {
        throw new IllegalStateException("expected cancellation before release");
      }
      CancellationScope.throwCanceled();
      return "unreachable";
    }

    @Override
    public void release() {
      released = true;
    }
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
                      .setCancellationType(NexusOperationCancellationType.WAIT_REQUESTED)
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
      var gate =
          Workflow.newActivityStub(
              GateActivities.class,
              ActivityOptions.newBuilder().setStartToCloseTimeout(Duration.ofSeconds(10)).build());
      gate.checkpoint(false);
      assertFalse(outcome.isCompleted(), "WAIT_REQUESTED must wait for delivery");
      gate.checkpoint(true);
      assertEquals("canceled", outcome.get());
      return stub.release(id);
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {service};
    }

    @Override
    public void prepareWorker(Worker worker) {
      worker.registerWorkflowImplementationTypes(HandlerWorkflowImpl.class);
      worker.registerActivitiesImplementations(
          new GateActivities() {
            @Override
            public void checkpoint(boolean release) {
              if (release) {
                service.allow.countDown();
              }
            }
          });
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
      assertEquals("canceled", runner.waitForRunResult(run, String.class));
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
      assertEquals(
          1,
          events.stream().filter(e -> e.hasNexusOperationCancelRequestedEventAttributes()).count());
      assertEquals(
          1,
          events.stream()
              .filter(e -> e.hasNexusOperationCancelRequestCompletedEventAttributes())
              .count());
      assertEquals(
          1, events.stream().filter(e -> e.hasNexusOperationCompletedEventAttributes()).count());
      for (var event : events) {
        assertFalse(event.hasNexusOperationFailedEventAttributes());
        assertFalse(event.hasNexusOperationTimedOutEventAttributes());
        assertFalse(event.hasNexusOperationCancelRequestFailedEventAttributes());
        if (event.hasNexusOperationCancelRequestCompletedEventAttributes()) {
          assertTrue(event.getEventId() < releaseId, "caller must wait for acknowledgement");
        }
        if (event.hasNexusOperationCanceledEventAttributes()) {
          assertTrue(event.getEventId() > releaseId, "caller must not wait for completion");
        }
        if (event.hasNexusOperationCompletedEventAttributes()) {
          assertEquals(
              releaseId, event.getNexusOperationCompletedEventAttributes().getScheduledEventId());
        }
      }
      runner.checkCurrentAndPastHistories(run);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    private final CountDownLatch allow = new CountDownLatch(1);

    @OperationImpl
    public OperationHandler<String, String> block() {
      OperationHandler<String, String> delegate =
          WorkflowRunOperation.fromWorkflowMethod(
              (context, details, id) ->
                  Nexus.getOperationContext()
                          .getWorkflowClient()
                          .newWorkflowStub(
                              HandlerWorkflow.class,
                              WorkflowOptions.newBuilder().setWorkflowId(id).build())
                      ::run);
      return new OperationHandler<String, String>() {
        @Override
        public OperationStartResult<String> start(
            OperationContext context, OperationStartDetails details, String id)
            throws OperationException {
          return delegate.start(context, details, id);
        }

        @Override
        public void cancel(OperationContext context, OperationCancelDetails details) {
          try {
            if (!allow.await(20, TimeUnit.SECONDS)) {
              throw new IllegalStateException("cancellation gate was not released");
            }
          } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new RuntimeException(e);
          }
          delegate.cancel(context, details);
        }
      };
    }

    @OperationImpl
    public OperationHandler<String, String> release() {
      return OperationHandler.sync(
          (context, details, id) -> {
            var stub = Nexus.getOperationContext().getWorkflowClient().newUntypedWorkflowStub(id);
            stub.signal("release");
            try {
              stub.getResult(String.class);
              throw new IllegalStateException("expected backing workflow cancellation");
            } catch (WorkflowFailedException e) {
              if (!(e.getCause() instanceof CanceledFailure)) {
                throw e;
              }
              return "canceled";
            }
          });
    }
  }
}
