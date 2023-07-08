package data_converter.binary;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;

import com.google.protobuf.util.JsonFormat;
import io.temporal.api.common.v1.Payload;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.nio.file.Files;
import java.nio.file.Paths;

@WorkflowInterface
public interface feature extends Feature {
  byte[] DEAD_BEEF = new byte[] {(byte) 0xde, (byte) 0xad, (byte) 0xbe, (byte) 0xef};

  @WorkflowMethod
  byte[] workflow();

  class Impl implements feature {
    /** run a workflow that returns binary value `0xdeadbeef` */
    @Override
    public byte[] workflow() {
      return DEAD_BEEF;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      // verify client result is binary `0xdeadbeef`
      var result = runner.waitForRunResult(run, byte[].class);
      assertArrayEquals(DEAD_BEEF, result);

      // get result payload of WorkflowExecutionCompleted event from workflow history
      var payload = runner.getWorkflowResultPayload(run);

      // load JSON payload from `./payload.json` and compare it to JSON representation of result
      // payload
      var content =
          Files.readAllBytes(
              Paths.get(
                  System.getProperty("user.dir"),
                  "..",
                  "features",
                  runner.featureInfo.dir,
                  "payload.json"));
      var builder = Payload.newBuilder();
      JsonFormat.parser().merge(new String(content), builder);
      var expected = builder.build();
      assertEquals(expected, payload);
    }
  }
}
