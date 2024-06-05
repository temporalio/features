package grpc_retry.server_unavailable_for_initiator;

import io.temporal.activity.ActivityInterface;
import io.temporal.client.WorkflowOptions;
import io.temporal.common.RetryOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import java.time.Duration;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  class Impl implements feature {
    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.proxyRejectAndAccept(
          Duration.ofSeconds(5), () -> feature.super.execute(runner));
    }

    @Override
    public void workflowOptions(WorkflowOptions.Builder builder) {
      builder.setRetryOptions(
          RetryOptions.newBuilder()
              .setInitialInterval(Duration.ofMillis(1))
              .setMaximumInterval(Duration.ofMillis(100))
              .setBackoffCoefficient(2.0)
              .validateBuildWithDefaults());
    }

    @Override
    public void workflow() {}

    @Override
    public boolean initiatorUsesProxy() {
      return true;
    }
  }
}
