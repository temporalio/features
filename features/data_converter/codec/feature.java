package data_converter.codec;

import static java.nio.charset.StandardCharsets.UTF_8;
import static org.junit.jupiter.api.Assertions.assertEquals;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.api.common.v1.Payload;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.CodecDataConverter;
import io.temporal.common.converter.DefaultDataConverter;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Message;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.util.Base64;
import java.util.Collections;

@WorkflowInterface
public interface feature extends Feature {

  ObjectMapper mapper = new ObjectMapper();
  Message expected = new Message(true);

  // An "echo" workflow
  @WorkflowMethod
  Message workflow(Message res);

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
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      CodecDataConverter converter =
          new CodecDataConverter(
              DefaultDataConverter.newDefaultInstance(),
              Collections.singletonList(new Base64Codec()));

      builder.setDataConverter(converter);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var result = runner.waitForRunResult(run, Message.class);
      assertEquals(expected, result);

      var payload = runner.getWorkflowResultPayload(run);

      var encoding = payload.getMetadataMap().get("encoding");
      assertEquals("my-encoding", encoding.toString(UTF_8));

      byte[] plainData = Base64.getDecoder().decode(new String(payload.getData().toByteArray()));
      Payload innerPayload = Payload.parseFrom(plainData);

      encoding = innerPayload.getMetadataMap().get("encoding");
      assertEquals("json/plain", encoding.toString(UTF_8));

      var resInHist = mapper.readValue(innerPayload.getData().toByteArray(), Message.class);
      assertEquals(result, resInHist);

      var payloadArg = runner.getWorkflowArgumentPayload(run);
      assertEquals(payload, payloadArg);
    }
  }
}
