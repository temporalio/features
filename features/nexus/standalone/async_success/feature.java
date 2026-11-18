package nexus.standalone.async_success;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.NexusOperationHandle;
import io.temporal.client.WorkflowOptions;
import io.temporal.nexus.Nexus;
import io.temporal.nexus.WorkflowRunOperation;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import nexus.standalone.StandaloneNexus;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod(name = "StandaloneNexusAsyncSuccess")
  String greet(String name);

  @Service(name = StandaloneNexus.SERVICE)
  interface TestService {
    @Operation(name = "async-success")
    String greet(String name);
  }

  class Impl implements feature {
    @Override
    public String greet(String name) {
      return "Hello from the workflow, " + name + "!";
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
    }

    @Override
    public Run execute(Runner runner) {
      var handle = StandaloneNexus.start(runner, TestService.class, TestService::greet);
      assertEquals("Hello from the workflow, world!", handle.getResult());
      checkLinks(runner, handle);
      return null;
    }

    private void checkLinks(Runner runner, NexusOperationHandle<String> handle) {
      var info = handle.describe().getRawInfo();
      assertEquals(1, info.getLinksCount(), "expected a link to the backing workflow");
      assertTrue(info.getLinks(0).hasWorkflowEvent());
      var workflowLink = info.getLinks(0).getWorkflowEvent();
      assertEquals(runner.config.namespace, workflowLink.getNamespace());
      assertFalse(workflowLink.getWorkflowId().isEmpty());
      assertFalse(workflowLink.getRunId().isEmpty());
      assertEquals(
          EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
          workflowLink.getEventRef().getEventType());

      try (var history =
          runner.client.streamHistory(workflowLink.getWorkflowId(), workflowLink.getRunId())) {
        var started = history.findFirst().orElseThrow();
        assertEquals(EventType.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED, started.getEventType());
        assertEquals(started.getEventId(), workflowLink.getEventRef().getEventId());

        var attrs = started.getWorkflowExecutionStartedEventAttributes();
        assertEquals(1, attrs.getCompletionCallbacksCount());
        var callback = attrs.getCompletionCallbacks(0);
        assertTrue(callback.hasNexus());
        assertEquals(1, callback.getLinksCount(), "expected a link back to the operation");
        assertTrue(callback.getLinks(0).hasNexusOperation());
        var operationLink = callback.getLinks(0).getNexusOperation();
        assertEquals(runner.config.namespace, operationLink.getNamespace());
        assertEquals(handle.getNexusOperationId(), operationLink.getOperationId());
        assertEquals(handle.getNexusOperationRunId(), operationLink.getRunId());
      }
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> greet() {
      return WorkflowRunOperation.fromWorkflowMethod(
          (context, details, name) ->
              Nexus.getOperationContext()
                      .getWorkflowClient()
                      .newWorkflowStub(
                          feature.class,
                          WorkflowOptions.newBuilder()
                              .setWorkflowId(details.getRequestId())
                              .build())
                  ::greet);
    }
  }
}
