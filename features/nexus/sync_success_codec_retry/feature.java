package nexus.sync_success_codec_retry;

import static io.temporal.sdkfeatures.NexusAssertions.assertHistory;
import static org.junit.jupiter.api.Assertions.assertEquals;

import data_converter.codec.Base64Codec;
import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.api.common.v1.Payload;
import io.temporal.api.enums.v1.EventType;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DefaultDataConverter;
import io.temporal.payload.codec.PayloadCodecException;
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
import java.util.concurrent.atomic.AtomicInteger;

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
    // Only the codec, handler, and result check use these counters, never the workflow.
    private final AtomicInteger handlerCalls = new AtomicInteger();
    private final AtomicInteger encodeAttempts = new AtomicInteger();

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
      assertEquals("result", stub.runOperation("input"));
      return "ok";
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      var codec =
          new Base64Codec() {
            @Override
            public List<Payload> encode(List<Payload> payloads) {
              for (var payload : payloads) {
                if (payload.getData().toStringUtf8().equals("\"result\"")
                    && encodeAttempts.incrementAndGet() == 1) {
                  throw new PayloadCodecException("one-shot result encoding failure");
                }
              }
              return super.encode(payloads);
            }
          };
      builder.setDataConverter(
          new CodecDataConverter(DefaultDataConverter.newDefaultInstance(), List.of(codec)));
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl(handlerCalls)};
    }

    @Override
    public Run execute(Runner runner) {
      handlerCalls.set(0);
      encodeAttempts.set(0);
      return runner.executeSingleWorkflow(null, runner.nexusEndpoint);
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      assertEquals("ok", runner.waitForRunResult(run, String.class));
      assertEquals(2, handlerCalls.get(), "handler calls");
      assertEquals(2, encodeAttempts.get(), "result encoding attempts");
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
    private final AtomicInteger calls;

    public TestServiceImpl(AtomicInteger calls) {
      this.calls = calls;
    }

    @OperationImpl
    public OperationHandler<String, String> runOperation() {
      return OperationHandler.sync(
          (ctx, details, input) -> {
            calls.incrementAndGet();
            return "result";
          });
    }
  }
}
