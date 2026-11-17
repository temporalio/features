package activity.standalone;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.api.enums.v1.ActivityExecutionStatus;
import io.temporal.client.ActivityClient;
import io.temporal.client.ActivityClientOptions;
import io.temporal.client.StartActivityOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import java.time.Duration;
import java.util.UUID;
import org.junit.jupiter.api.Assertions;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  @ActivityMethod
  String echo(String value);

  class Impl implements feature {
    @Override
    public void workflow() {}

    @Override
    public String echo(String value) {
      return value;
    }

    @Override
    public Run execute(Runner runner) {
      var client =
          ActivityClient.newInstance(
              runner.service,
              ActivityClientOptions.newBuilder().setNamespace(runner.config.namespace).build());

      var handle =
          client.start(
              feature.class,
              feature::echo,
              StartActivityOptions.newBuilder()
                  .setId(UUID.randomUUID().toString())
                  .setTaskQueue(runner.config.taskQueue)
                  .setStartToCloseTimeout(Duration.ofMinutes(1))
                  .build(),
              "hello");

      Assertions.assertEquals("hello", handle.getResult());
      Assertions.assertEquals(
          ActivityExecutionStatus.ACTIVITY_EXECUTION_STATUS_COMPLETED,
          handle.describe().getStatus());
      return null;
    }
  }
}
