package data_converter.empty;

import io.temporal.api.common.v1.Payload;
import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;

import java.time.Duration;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

import java.nio.file.Files;
import java.nio.file.Paths;

import com.google.protobuf.util.JsonFormat;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {

  @ActivityMethod
  void activity(String input);

  class Impl implements feature {
    /**
     * run a workflow that calls an activity with a null parameter.
     */
    @Override
    public void workflow() {
      var activities = activities(feature.class, builder -> builder
              .setStartToCloseTimeout(Duration.ofMinutes(1)));
      activities.activity(null);
    }

    @Override
    public void activity(String input) {
      // check the null input is serialized correctly
      if (input != null) {
        throw ApplicationFailure.newNonRetryableFailure("Activity input should be null", "BadResult");
      }
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      // verify the workflow returns nothing
      var result = runner.waitForRunResult(run, Object.class);
      assertNull(result);

      // get result payload of ActivityTaskScheduled event from workflow history
      var history = runner.getWorkflowHistory(run);
      var event = history.getEventsList().stream().filter(e -> e.hasActivityTaskScheduledEventAttributes()).findFirst();
      var payload = event.get().getActivityTaskScheduledEventAttributes().getInput().getPayloads(0);

      // load JSON payload from `./payload.json` and compare it to JSON representation of result payload
      var content = Files.readAllBytes(Paths.get(System.getProperty("user.dir"), "..", "features", runner.featureInfo.dir, "payload.json"));
      var builder = Payload.newBuilder();
      JsonFormat.parser().merge(new String(content), builder);
      var expected = builder.build();
      assertEquals(expected, payload);
    }
  }
}