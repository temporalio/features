package tracing.encoded_headers;

import static org.junit.jupiter.api.Assertions.*;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.protobuf.ByteString;
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.Scope;
import io.opentelemetry.context.propagation.ContextPropagators;
import io.opentelemetry.opentracingshim.OpenTracingShim;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.temporal.api.common.v1.Payload;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DefaultDataConverter;
import io.temporal.opentracing.OpenTracingClientInterceptor;
import io.temporal.opentracing.OpenTracingOptions;
import io.temporal.opentracing.OpenTracingWorkerInterceptor;
import io.temporal.payload.codec.PayloadCodec;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.util.*;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod(name = "workflow")
  Map<String, String> workflow(String message);

  class Impl implements feature {
    static final OpenTelemetry telemetry =
        OpenTelemetrySdk.builder()
            .setTracerProvider(SdkTracerProvider.builder().build())
            .setPropagators(ContextPropagators.create(W3CTraceContextPropagator.getInstance()))
            .build();
    static final io.opentracing.Tracer tracer = OpenTracingShim.createTracerShim(telemetry);
    final OpenTracingOptions options = tracingOptions();

    static OpenTracingOptions tracingOptions() {
      var builder = OpenTracingOptions.newBuilder().setTracer(tracer);
      try {
        // Older releases can still compile the ordinary suite; a selected participant must opt in.
        builder.getClass().getMethod("setEncodeHeaders", boolean.class).invoke(builder, true);
      } catch (ReflectiveOperationException e) {
        throw new IllegalStateException(
            "Java tracing requires producer-specific setEncodeHeaders(true)", e);
      }
      return builder.build();
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setDataConverter(
          new CodecDataConverter(
              DefaultDataConverter.newDefaultInstance(), List.of(new WrappingCodec())));
      builder.setInterceptors(new OpenTracingClientInterceptor(options));
    }

    @Override
    public void workerFactoryOptions(WorkerFactoryOptions.Builder builder) {
      builder.setWorkerInterceptors(new OpenTracingWorkerInterceptor(options));
    }

    @Override
    public Map<String, String> workflow(String message) {
      var span = Span.current().getSpanContext();
      assertTrue(span.isValid(), "No active workflow tracing span");
      return Map.of("message", message, "traceId", span.getTraceId());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var parent =
          telemetry.getTracer("encoded-headers-feature").spanBuilder("java-caller").startSpan();
      try (Scope ignored = parent.makeCurrent()) {
        var run =
            runner.executeSingleWorkflow(
                io.temporal.client.WorkflowOptions.newBuilder()
                    .setTaskQueue(runner.config.taskQueue)
                    .setWorkflowExecutionTimeout(java.time.Duration.ofSeconds(15))
                    .build(),
                "hello");
        Map<?, ?> result;
        try {
          result = runner.waitForRunResult(run, Map.class);
        } finally {
          for (var event : runner.getWorkflowHistory(run).getEventsList()) {
            assertFalse(event.hasWorkflowTaskFailedEventAttributes(), event.toString());
          }
        }
        var header =
            runner
                .getWorkflowHistory(run)
                .getEvents(0)
                .getWorkflowExecutionStartedEventAttributes()
                .getHeader()
                .getFieldsOrThrow("_tracer-data");
        var decoded = new WrappingCodec().decode(List.of(header)).get(0);
        Map<String, String> carrier =
            new ObjectMapper()
                .readValue(
                    decoded.getData().toByteArray(), new TypeReference<Map<String, String>>() {});
        assertEquals(
            parent.getSpanContext().getTraceId(), carrier.get("traceparent").split("-")[1]);
        assertEquals(
            Map.of("message", "hello", "traceId", parent.getSpanContext().getTraceId()), result);
        return null;
      } finally {
        parent.end();
      }
    }
  }

  class WrappingCodec implements PayloadCodec {
    static final ByteString ENCODING = ByteString.copyFromUtf8("binary/encrypted");

    static byte[] xor(byte[] data) {
      for (int i = 0; i < data.length; i++) data[i] ^= 0x5a;
      return data;
    }

    @Override
    public List<Payload> encode(List<Payload> payloads) {
      var result = new ArrayList<Payload>();
      for (var payload : payloads) {
        assertNotEquals(
            ENCODING, payload.getMetadataMap().get("encoding"), "Payload encoded twice");
        result.add(
            Payload.newBuilder()
                .putMetadata("encoding", ENCODING)
                .setData(ByteString.copyFrom(xor(payload.toByteArray())))
                .build());
      }
      return result;
    }

    @Override
    public List<Payload> decode(List<Payload> payloads) {
      var result = new ArrayList<Payload>();
      for (var payload : payloads) {
        assertEquals(ENCODING, payload.getMetadataMap().get("encoding"));
        try {
          result.add(Payload.parseFrom(xor(payload.getData().toByteArray())));
        } catch (com.google.protobuf.InvalidProtocolBufferException e) {
          throw new IllegalArgumentException("Invalid wrapped payload", e);
        }
      }
      return result;
    }
  }
}
