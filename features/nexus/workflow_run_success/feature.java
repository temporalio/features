package nexus.workflow_run_success;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.common.v1.Link;
import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.api.enums.v1.EventType;
import io.temporal.api.history.v1.History;
import io.temporal.api.history.v1.HistoryEvent;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.internal.client.WorkflowClientHelper;
import io.temporal.nexus.Nexus;
import io.temporal.nexus.WorkflowHandle;
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
import java.util.List;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service
  interface TestService {
    @Operation(name = "AsyncWorkflowOperation")
    String asyncWorkflowOperation(String name);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String run(String name);
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    @Override
    public String run(String name) {
      return "Hello, " + name + "!";
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
      return stub.asyncWorkflowOperation("world");
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
      assertEquals("Hello, world!", result);
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      // Async (workflow-run) Nexus operations should transition Scheduled -> Started -> Completed.
      var events = runner.getWorkflowHistory(run).getEventsList();
      var scheduled = findEvent(events, e -> e.hasNexusOperationScheduledEventAttributes());
      assertNotNull(scheduled, "expected NexusOperationScheduled event in history");
      var started = findEvent(events, e -> e.hasNexusOperationStartedEventAttributes());
      assertNotNull(started, "expected NexusOperationStarted event in history");
      var completed = findEvent(events, e -> e.hasNexusOperationCompletedEventAttributes());
      assertNotNull(completed, "expected NexusOperationCompleted event in history");

      // The caller's NexusOperationStarted event must link to the handler workflow's
      // WorkflowExecutionStarted event.
      var handlerLink =
          findWorkflowEventLink(
              started.getLinksList(), EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED);
      assertNotNull(
          handlerLink,
          "NexusOperationStarted is missing a link to the handler WorkflowExecutionStarted event");
      assertEquals(runner.config.namespace, handlerLink.getNamespace());
      // WorkflowExecutionStarted is always event ID 1.
      assertEquals(1L, handlerLink.getEventRef().getEventId());
      // The handler workflow ID is set to the Nexus operation request ID by the operation impl.
      assertEquals(
          scheduled.getNexusOperationScheduledEventAttributes().getRequestId(),
          handlerLink.getWorkflowId());

      // The handler workflow's WorkflowExecutionStarted event carries the Nexus completion
      // callback, whose link points back to the caller's NexusOperationScheduled event.
      // (Nexus links on the started event itself are deduped against the callback link.)
      var handlerExec =
          WorkflowExecution.newBuilder()
              .setWorkflowId(handlerLink.getWorkflowId())
              .setRunId(handlerLink.getRunId())
              .build();
      var handlerEventIter =
          WorkflowClientHelper.getHistory(
              runner.service, runner.config.namespace, handlerExec, runner.config.metricsScope);
      var handlerEvents =
          History.newBuilder().addAllEvents(() -> handlerEventIter).build().getEventsList();
      var handlerStarted =
          findEvent(handlerEvents, e -> e.hasWorkflowExecutionStartedEventAttributes());
      assertNotNull(handlerStarted, "expected WorkflowExecutionStarted event in handler history");
      var attrs = handlerStarted.getWorkflowExecutionStartedEventAttributes();
      // Cross-check the run ID embedded in the caller's link against the handler's own attrs.
      assertEquals(attrs.getFirstExecutionRunId(), handlerLink.getRunId());
      assertTrue(
          attrs.getCompletionCallbacksCount() > 0,
          "handler WorkflowExecutionStarted has no completion callbacks");
      var callerLink =
          findWorkflowEventLink(
              attrs.getCompletionCallbacks(0).getLinksList(),
              EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED);
      assertNotNull(
          callerLink,
          "handler completion callback is missing a link to the caller NexusOperationScheduled event");
      assertEquals(runner.config.namespace, callerLink.getNamespace());
      assertEquals(run.execution.getWorkflowId(), callerLink.getWorkflowId());
      assertEquals(run.execution.getRunId(), callerLink.getRunId());
      assertEquals(scheduled.getEventId(), callerLink.getEventRef().getEventId());
    }

    private static HistoryEvent findEvent(
        List<HistoryEvent> events, java.util.function.Predicate<HistoryEvent> cond) {
      return events.stream().filter(cond).findFirst().orElse(null);
    }

    private static Link.WorkflowEvent findWorkflowEventLink(List<Link> links, EventType type) {
      for (Link l : links) {
        if (!l.hasWorkflowEvent()) {
          continue;
        }
        var we = l.getWorkflowEvent();
        if (we.getEventRef().getEventType() == type) {
          return we;
        }
      }
      return null;
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> asyncWorkflowOperation() {
      return WorkflowRunOperation.fromWorkflowHandle(
          (context, details, name) -> {
            WorkflowClient client = Nexus.getOperationContext().getWorkflowClient();
            return WorkflowHandle.fromWorkflowMethod(
                client.newWorkflowStub(
                        HandlerWorkflow.class,
                        WorkflowOptions.newBuilder().setWorkflowId(details.getRequestId()).build())
                    ::run,
                name);
          });
    }
  }
}
