package nexus.sync_success_codec;

import static io.temporal.sdkfeatures.NexusAssertions.assertHistory;
import static org.junit.jupiter.api.Assertions.assertEquals;

import data_converter.codec.Base64Codec;
import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DefaultDataConverter;
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
      return Workflow.newNexusServiceStub(TestService.class, options).runOperation("input");
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(
          new CodecDataConverter(
              DefaultDataConverter.newDefaultInstance(), List.of(new Base64Codec())));
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
      assertEquals("result", runner.waitForRunResult(run, String.class));
    }

    @Override
    public void checkHistory(Runner runner, Run run) throws Exception {
      for (var event : runner.getWorkflowHistory(run).getEventsList()) {
        if (event.hasNexusOperationScheduledEventAttributes()) {
          assertEquals(
              Base64Codec.METADATA_ENCODING,
              event
                  .getNexusOperationScheduledEventAttributes()
                  .getInput()
                  .getMetadataMap()
                  .get("encoding"));
        }
        if (event.hasNexusOperationCompletedEventAttributes()) {
          assertEquals(
              Base64Codec.METADATA_ENCODING,
              event
                  .getNexusOperationCompletedEventAttributes()
                  .getResult()
                  .getMetadataMap()
                  .get("encoding"));
        }
      }
      assertHistory(
          runner,
          run,
          EventType.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
          EventType.EVENT_TYPE_NEXUS_OPERATION_COMPLETED);
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> runOperation() {
      return OperationHandler.sync((ctx, details, input) -> "result");
    }
  }
}
