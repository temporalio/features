package nexus.sync_operation_error_codec;

import static io.temporal.sdkfeatures.NexusAssertions.*;
import static org.junit.jupiter.api.Assertions.*;

import data_converter.codec.Base64Codec;
import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.common.v1.Payload;
import io.temporal.api.enums.v1.EventType;
import io.temporal.api.failure.v1.Failure;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DefaultDataConverter;
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
import java.util.List;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow(String endpoint);

  @Service(name = "codec-service")
  interface TestService {
    @Operation(name = "run")
    String runOperation(String input);
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
                      .build())
              .build();
      var stub = Workflow.newNexusServiceStub(TestService.class, options);
      try {
        stub.runOperation("input");
        fail("expected an operation failure");
      } catch (NexusOperationFailure e) {
        assertOperationFailure(e, endpoint, "codec-service", "run", "");
        assertInstanceOf(ApplicationFailure.class, e.getCause());
        var wrapper = (ApplicationFailure) e.getCause();
        assertEquals("OperationError", wrapper.getType());
        assertEquals("operation failed", wrapper.getOriginalMessage());
        assertApplicationCause(wrapper.getCause(), "CodecFailure", "deliberate failure", "detail");
      }
      return "ok";
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(
          new CodecDataConverter(
              DefaultDataConverter.newDefaultInstance(), List.of(new Base64Codec()), true));
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
    }

    @Override
    public Run execute(Runner runner) {
      return runner.executeSingleWorkflow(null, runner.nexusEndpoint);
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      assertEquals("ok", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      for (var event : runner.getWorkflowHistory(run).getEventsList()) {
        if (event.hasNexusOperationScheduledEventAttributes()) {
          assertEncoded(event.getNexusOperationScheduledEventAttributes().getInput());
        }
        if (event.hasNexusOperationFailedEventAttributes()) {
          // NexusOperationFailure -> OperationError -> application cause.
          var wrapper = event.getNexusOperationFailedEventAttributes().getFailure().getCause();
          var cause = wrapper.getCause();
          assertCommonAttributesEncoded(wrapper);
          assertCommonAttributesEncoded(cause);
          var details = cause.getApplicationFailureInfo().getDetails();
          assertEquals(1, details.getPayloadsCount());
          assertEncoded(details.getPayloads(0));
        }
      }
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_FAILED);
    }

    private static void assertCommonAttributesEncoded(Failure failure) {
      assertEquals("Encoded failure", failure.getMessage());
      assertEquals("", failure.getStackTrace());
      assertEncoded(failure.getEncodedAttributes());
    }

    private static void assertEncoded(Payload payload) {
      assertEquals(Base64Codec.METADATA_ENCODING, payload.getMetadataMap().get("encoding"));
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> runOperation() {
      return OperationHandler.sync(
          (ctx, details, input) -> {
            throw OperationException.failed(
                "operation failed",
                ApplicationFailure.newFailure("deliberate failure", "CodecFailure", "detail"));
          });
    }
  }
}
