package serialization_context.sercontext;

import com.google.protobuf.ByteString;
import io.temporal.api.common.v1.Payload;
import io.temporal.api.common.v1.Payloads;
import io.temporal.api.history.v1.History;
import io.temporal.api.history.v1.HistoryEvent;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DataConverter;
import io.temporal.common.converter.DefaultDataConverter;
import io.temporal.payload.context.ActivitySerializationContext;
import io.temporal.payload.context.SerializationContext;
import io.temporal.payload.context.WorkflowSerializationContext;
import java.util.Collections;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Predicate;

/**
 * Context aware converters shared by the serialization_context features, plus history helpers used
 * to assert on the recorded context.
 */
public final class SerContext {

  public static final String METADATA_KEY = "ctx-signature";
  public static final String NO_CONTEXT = "none";
  public static final String DEFAULT_FAILURE_SOURCE = "JavaSDK";

  /**
   * Every signature the codec has been asked to encode or decode with. Worker and client share a
   * process here, so this is how a feature asserts on a context whose payload never shows up in
   * history.
   */
  public static final Set<String> OBSERVED_SIGNATURES = ConcurrentHashMap.newKeySet();

  private SerContext() {}

  public static String workflowSignature(String namespace, String workflowId) {
    return "wf|" + namespace + "|" + workflowId;
  }

  public static String activitySignature(
      String namespace,
      String workflowId,
      String workflowType,
      String activityType,
      String activityTaskQueue,
      boolean local) {
    return "act|"
        + namespace
        + "|"
        + workflowId
        + "|"
        + workflowType
        + "|"
        + activityType
        + "|"
        + activityTaskQueue
        + "|"
        + local;
  }

  public static String signature(SerializationContext context) {
    if (context instanceof WorkflowSerializationContext) {
      WorkflowSerializationContext workflow = (WorkflowSerializationContext) context;
      return workflowSignature(workflow.getNamespace(), workflow.getWorkflowId());
    }
    if (context instanceof ActivitySerializationContext) {
      ActivitySerializationContext activity = (ActivitySerializationContext) context;
      return activitySignature(
          activity.getNamespace(),
          activity.getWorkflowId(),
          activity.getWorkflowType(),
          activity.getActivityType(),
          activity.getActivityTaskQueue(),
          activity.isLocal());
    }
    return NO_CONTEXT;
  }

  public static DataConverter dataConverter() {
    return new CodecDataConverter(
        DefaultDataConverter.newDefaultInstance().withFailureConverter(new SigningFailureConverter()),
        Collections.singletonList(new SigningCodec()));
  }

  public static String signatureOf(Payload payload) {
    ByteString signature = payload.getMetadataMap().get(METADATA_KEY);
    return signature == null ? "" : signature.toStringUtf8();
  }

  public static String firstSignature(Payloads payloads) {
    return signatureOf(payloads.getPayloads(0));
  }

  public static HistoryEvent findEvent(
      History history, String name, Predicate<HistoryEvent> predicate) {
    return history.getEventsList().stream()
        .filter(predicate)
        .findFirst()
        .orElseThrow(() -> new AssertionError("no " + name + " event in history"));
  }
}
