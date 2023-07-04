package data_converter.json;

import static java.nio.charset.StandardCharsets.UTF_8;
import static org.junit.jupiter.api.Assertions.assertEquals;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Message;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface feature extends Feature {

  ObjectMapper mapper = new ObjectMapper();
  Message expected = new Message(true);

  // An "echo" workflow
  @WorkflowMethod
  public Message workflow(Message res);

  class Impl implements feature {

    @Override
    public Message workflow(Message res) {
      return res;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(null, expected);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var result = runner.waitForRunResult(run, Message.class);
      assertEquals(expected, result);

      var payload = runner.getWorkflowResultPayload(run);

      var encoding = payload.getMetadataMap().get("encoding");
      assertEquals("json/plain", encoding.toString(UTF_8));

      var resInHist = mapper.readValue(payload.getData().toByteArray(), Message.class);
      assertEquals(result, resInHist);

      var payloadArg = runner.getWorkflowArgumentPayload(run);
      assertEquals(payload, payloadArg);
    }
  }
}
