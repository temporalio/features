package external_storage.s3_interop;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.protobuf.ByteString;
import com.google.protobuf.util.JsonFormat;
import io.temporal.api.common.v1.Payload;
import io.temporal.api.common.v1.Payloads;
import io.temporal.api.history.v1.HistoryEvent;
import io.temporal.api.sdk.v1.ExternalStorageReference;
import io.temporal.api.workflowservice.v1.SignalWorkflowExecutionRequest;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.payload.storage.ExternalStorage;
import io.temporal.payload.storage.s3driver.S3StorageDriver;
import io.temporal.payload.storage.s3driver.awssdkv2.S3AsyncClientAdapter;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.math.BigInteger;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Base64;
import java.util.Map;
import software.amazon.awssdk.core.async.AsyncRequestBody;
import software.amazon.awssdk.core.async.AsyncResponseTransformer;
import software.amazon.awssdk.services.s3.S3AsyncClient;

@WorkflowInterface
public interface feature extends Feature {
  String BUCKET = System.getenv("TEMPORAL_FEATURES_S3_BUCKET");
  String ENDPOINT = System.getenv("TEMPORAL_FEATURES_S3_ENDPOINT");
  String DRIVER_NAME = "aws.s3driver";
  String REFERENCE_TYPE = "temporal.api.sdk.v1.ExternalStorageReference";
  String SIGNAL = "foreign";
  String FOREIGN_VALUE = "hello";
  String FOREIGN_PAYLOAD = "ChYKCGVuY29kaW5nEgpqc29uL3BsYWluEgciaGVsbG8i";
  String FOREIGN_HASH = "1b8435174847de88db3a8c5a764295d62add152743b3a9a730796a4c090caf8c";
  String FOREIGN_KEY = "v0/d/sha256/" + FOREIGN_HASH;
  String INPUT_HASH = "8016abfa3587d601e521edf45b84c3fca4ef12f10d6b470d4c4aa0eab0f82f1a";
  String INPUT = "x".repeat(300 * 1024);
  S3AsyncClient S3 = BUCKET == null ? null : Impl.newS3Client();

  @WorkflowMethod
  String workflow(String value);

  @SignalMethod(name = SIGNAL)
  void foreign(String value);

  class Impl implements feature {
    private String foreign;

    @Override
    public String workflow(String value) {
      Workflow.await(() -> foreign != null);
      if (!FOREIGN_VALUE.equals(foreign)) {
        throw ApplicationFailure.newFailure(
            "foreign payload is " + foreign + ", expected " + FOREIGN_VALUE, "Mismatch");
      }
      return value;
    }

    @Override
    public void foreign(String value) {
      foreign = value;
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      if (S3 == null) {
        return;
      }
      builder.setExternalStorage(
          ExternalStorage.newBuilder()
              .setDriver(
                  S3StorageDriver.newBuilder()
                      .setClient(new S3AsyncClientAdapter(S3))
                      .setBucket(BUCKET)
                      .build())
              .build());
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      if (S3 == null) {
        runner.skip("TEMPORAL_FEATURES_S3_BUCKET is not set");
      }

      S3.putObject(
              b -> b.bucket(BUCKET).key(FOREIGN_KEY),
              AsyncRequestBody.fromBytes(Base64.getDecoder().decode(FOREIGN_PAYLOAD)))
          .get();

      var run = runner.executeSingleWorkflow(null, INPUT);

      var reference =
          ExternalStorageReference.newBuilder()
              .setDriverName(DRIVER_NAME)
              .putAllClaimData(
                  Map.of(
                      "bucket", BUCKET,
                      "key", FOREIGN_KEY,
                      "hash_algorithm", "sha256",
                      "hash_value", FOREIGN_HASH))
              .build();
      var payload =
          Payload.newBuilder()
              .putMetadata("encoding", ByteString.copyFromUtf8("json/protobuf"))
              .putMetadata("messageType", ByteString.copyFromUtf8(REFERENCE_TYPE))
              .setData(ByteString.copyFromUtf8(JsonFormat.printer().print(reference)))
              .build();
      runner
          .service
          .blockingStub()
          .signalWorkflowExecution(
              SignalWorkflowExecutionRequest.newBuilder()
                  .setNamespace(runner.config.namespace)
                  .setWorkflowExecution(run.execution)
                  .setSignalName(SIGNAL)
                  .setInput(Payloads.newBuilder().addPayloads(payload))
                  .build());
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      assertEquals(INPUT, runner.waitForRunResult(run, String.class));

      var history = runner.getWorkflowHistory(run);
      var started =
          history.getEventsList().stream()
              .filter(HistoryEvent::hasWorkflowExecutionStartedEventAttributes)
              .findFirst()
              .get()
              .getWorkflowExecutionStartedEventAttributes();
      var completed =
          history.getEventsList().stream()
              .filter(HistoryEvent::hasWorkflowExecutionCompletedEventAttributes)
              .findFirst()
              .get()
              .getWorkflowExecutionCompletedEventAttributes();

      var prefix =
          "v0/ns/"
              + runner.config.namespace
              + "/wt/"
              + started.getWorkflowType().getName()
              + "/wi/"
              + run.execution.getWorkflowId();
      var inputKey = prefix + "/ri/null/d/sha256/" + INPUT_HASH;
      checkReference(started.getInput().getPayloads(0), inputKey);
      checkReference(
          completed.getResult().getPayloads(0),
          prefix + "/ri/" + run.execution.getRunId() + "/d/sha256/" + INPUT_HASH);

      var object =
          S3.getObject(b -> b.bucket(BUCKET).key(inputKey), AsyncResponseTransformer.toBytes())
              .get()
              .asByteArray();
      var digest = MessageDigest.getInstance("SHA-256").digest(object);
      assertEquals(INPUT_HASH, String.format("%064x", new BigInteger(1, digest)));
    }

    private static void checkReference(Payload payload, String key) throws Exception {
      assertEquals("json/protobuf", payload.getMetadataOrThrow("encoding").toStringUtf8());
      assertEquals(REFERENCE_TYPE, payload.getMetadataOrThrow("messageType").toStringUtf8());
      assertTrue(payload.getExternalPayloadsCount() > 0);

      var reference = ExternalStorageReference.newBuilder();
      JsonFormat.parser()
          .ignoringUnknownFields()
          .merge(payload.getData().toString(StandardCharsets.UTF_8), reference);
      assertEquals(DRIVER_NAME, reference.getDriverName());
      assertEquals(
          Map.of(
              "bucket", BUCKET,
              "key", key,
              "hash_algorithm", "sha256",
              "hash_value", INPUT_HASH),
          reference.getClaimDataMap());
    }

    private static S3AsyncClient newS3Client() {
      var builder = S3AsyncClient.builder();
      if (ENDPOINT != null) {
        builder.endpointOverride(URI.create(ENDPOINT)).forcePathStyle(true);
      }
      return builder.build();
    }
  }
}
