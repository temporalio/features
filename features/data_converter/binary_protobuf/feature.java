package data_converter.binary_protobuf;

import static java.nio.charset.StandardCharsets.UTF_8;
import static org.junit.jupiter.api.Assertions.assertEquals;

import com.google.protobuf.ByteString;
import io.temporal.api.common.v1.DataBlob;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.common.converter.DefaultDataConverter;
import io.temporal.common.converter.NullPayloadConverter;
import io.temporal.common.converter.PayloadConverter;
import io.temporal.common.converter.ProtobufPayloadConverter;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface feature extends Feature {

  static byte[] deadbeef = new byte[] {(byte) 0xde, (byte) 0xad, (byte) 0xbe, (byte) 0xef};
  DataBlob expected = DataBlob.newBuilder().setData(ByteString.copyFrom(deadbeef)).build();

  // An "echo" workflow
  @WorkflowMethod
  public DataBlob workflow(DataBlob res);

  class Impl implements feature {

    @Override
    public DataBlob workflow(DataBlob res) {
      return res;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      return runner.executeSingleWorkflow(null, expected);
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      PayloadConverter[] converters = {new NullPayloadConverter(), new ProtobufPayloadConverter()};
      builder.setDataConverter(new DefaultDataConverter(converters));
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var result = runner.waitForRunResult(run, DataBlob.class);
      assertEquals(expected, result);

      var payload = runner.getWorkflowResultPayload(run);

      var encoding = payload.getMetadataMap().get("encoding");
      assertEquals("binary/protobuf", encoding.toString(UTF_8));

      var messageType = payload.getMetadataMap().get("messageType");
      assertEquals("temporal.api.common.v1.DataBlob", messageType.toString(UTF_8));

      var resInHist = DataBlob.newBuilder().mergeFrom(payload.getData()).build();
      assertEquals(result, resInHist);

      var payloadArg = runner.getWorkflowArgumentPayload(run);
      assertEquals(payload, payloadArg);
    }
  }
}
