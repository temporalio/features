package nexus.async_success;

import static io.temporal.sdkfeatures.NexusAssertions.assertHistory;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
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
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.ApplicationFailure;
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
import java.util.ArrayList;
import java.util.List;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service
  interface TestService {
    @Operation
    String asyncOperation(String name);
  }

  @WorkflowInterface
  interface HandlerWorkflow {
    @WorkflowMethod
    String handlerWorkflow(String name);
  }

  class HandlerWorkflowImpl implements HandlerWorkflow {
    @Override
    public String handlerWorkflow(String name) {
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
      var handle = Workflow.startNexusOperation(stub::asyncOperation, "world");
      var execution = handle.getExecution().get();
      if (execution.getOperationToken().orElse("").isEmpty()) {
        throw ApplicationFailure.newNonRetryableFailure(
            "expected a non-empty operation token", "AssertionFailure");
      }
      return handle.getResult().get();
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
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_STARTED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_COMPLETED);

      var callerHistory = runner.getWorkflowHistory(run);
      var scheduled = historyEvent(callerHistory, EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED);
      var started = historyEvent(callerHistory, EventType.EVENT_TYPE_NEXUS_OPERATION_STARTED);

      // The handler uses the operation request ID as its workflow ID. Resolve the
      // backing run independently of the link under test.
      var handlerId = scheduled.getNexusOperationScheduledEventAttributes().getRequestId();
      assertFalse(
          handlerId.isEmpty(), "expected a request ID on the NexusOperationScheduled event");
      var description = runner.client.newUntypedWorkflowStub(handlerId).describe();
      var handler = description.getExecution();
      var handlerHistory = runner.client.fetchHistory(handlerId, handler.getRunId()).getHistory();
      var handlerStarted =
          historyEvent(handlerHistory, EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED);

      assertWorkflowLink(
          "NexusOperationStarted",
          started.getLinksList(),
          runner.config.namespace,
          handler,
          handlerStarted);

      // Servers that deduplicate Nexus callback links expose the backlink on
      // the completion callback instead of on WorkflowExecutionStarted.
      var handlerLinks = new ArrayList<>(handlerStarted.getLinksList());
      for (var info : description.getRawDescription().getCallbacksList()) {
        var callback = info.getCallback();
        if (callback.hasNexus()) {
          handlerLinks.addAll(callback.getLinksList());
        }
      }
      assertWorkflowLink(
          "backing workflow start or Nexus completion callback",
          handlerLinks,
          runner.config.namespace,
          run.execution,
          scheduled);
    }

    private static HistoryEvent historyEvent(History history, EventType type) {
      return history.getEventsList().stream()
          .filter(event -> event.getEventType() == type)
          .findFirst()
          .orElseThrow(() -> new AssertionError("expected history event " + type));
    }

    private static void assertWorkflowLink(
        String source,
        List<Link> links,
        String namespace,
        WorkflowExecution target,
        HistoryEvent targetEvent) {
      assertTrue(
          links.stream()
              .filter(Link::hasWorkflowEvent)
              .map(Link::getWorkflowEvent)
              .anyMatch(
                  link -> {
                    var ref = link.getEventRef();
                    return link.getNamespace().equals(namespace)
                        && link.getWorkflowId().equals(target.getWorkflowId())
                        && link.getRunId().equals(target.getRunId())
                        && ref.getEventType() == targetEvent.getEventType()
                        // A link to the workflow's unique start event may omit its event ID.
                        && (ref.getEventId() == targetEvent.getEventId()
                            || (ref.getEventId() == 0
                                && targetEvent.getEventType()
                                    == EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED));
                  }),
          () ->
              String.format(
                  "expected %s to link to %s event %d in %s/%s/%s, got %s",
                  source,
                  targetEvent.getEventType(),
                  targetEvent.getEventId(),
                  namespace,
                  target.getWorkflowId(),
                  target.getRunId(),
                  links));
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> asyncOperation() {
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
