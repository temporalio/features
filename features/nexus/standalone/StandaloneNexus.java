package nexus.standalone;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.fail;

import io.grpc.Status;
import io.temporal.client.NexusClient;
import io.temporal.client.NexusClientOptions;
import io.temporal.client.NexusOperationHandle;
import io.temporal.client.StartNexusOperationOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.Functions;
import java.time.Duration;
import java.util.UUID;

public final class StandaloneNexus {
  public static final String SERVICE = "standalone-nexus";
  public static final String FAILURE_TYPE = "StandaloneNexusTestFailure";

  private StandaloneNexus() {}

  public static <S> NexusOperationHandle<String> start(
      Runner runner, Class<S> service, Functions.Func2<S, String, String> operation) {
    var client =
        NexusClient.newInstance(
            runner.service,
            NexusClientOptions.newBuilder().setNamespace(runner.config.namespace).build());
    try {
      return client
          .newNexusServiceClient(service, runner.nexusEndpoint)
          .start(
              operation,
              StartNexusOperationOptions.newBuilder()
                  .setId(UUID.randomUUID().toString())
                  .setScheduleToCloseTimeout(Duration.ofMinutes(1))
                  .build(),
              "world");
    } catch (RuntimeException e) {
      if (Status.fromThrowable(e).getCode() == Status.Code.UNIMPLEMENTED) {
        runner.skip("server does not implement the standalone Nexus API");
      }
      throw e;
    }
  }

  public static void assertApplicationFailure(Throwable error, String message) {
    // A synchronous operation failure can wrap the application failure.
    for (var cause = error; cause != null; cause = cause.getCause()) {
      if (cause instanceof ApplicationFailure) {
        var failure = (ApplicationFailure) cause;
        if (FAILURE_TYPE.equals(failure.getType())) {
          assertEquals(message, failure.getOriginalMessage());
          return;
        }
      }
    }
    fail("Expected application failure of type " + FAILURE_TYPE, error);
  }
}
