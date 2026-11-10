package serialization_context.sercontext;

import com.google.protobuf.ByteString;
import io.temporal.api.common.v1.Payload;
import io.temporal.payload.codec.PayloadCodec;
import io.temporal.payload.codec.PayloadCodecException;
import io.temporal.payload.context.SerializationContext;
import java.util.ArrayList;
import java.util.List;
import javax.annotation.Nonnull;

/**
 * Stamps the signature of its serialization context onto every payload it encodes and refuses to
 * decode a payload encoded under a different context.
 */
public class SigningCodec implements PayloadCodec {

  private final String signature;

  public SigningCodec() {
    this(SerContext.NO_CONTEXT);
  }

  private SigningCodec(String signature) {
    this.signature = signature;
  }

  @Override
  public PayloadCodec withContext(@Nonnull SerializationContext context) {
    return new SigningCodec(SerContext.signature(context));
  }

  @Nonnull
  @Override
  public List<Payload> encode(@Nonnull List<Payload> payloads) {
    SerContext.OBSERVED_SIGNATURES.add(signature);
    List<Payload> result = new ArrayList<>(payloads.size());
    for (Payload payload : payloads) {
      result.add(
          payload.toBuilder()
              .putMetadata(SerContext.METADATA_KEY, ByteString.copyFromUtf8(signature))
              .build());
    }
    return result;
  }

  @Nonnull
  @Override
  public List<Payload> decode(@Nonnull List<Payload> payloads) {
    SerContext.OBSERVED_SIGNATURES.add(signature);
    List<Payload> result = new ArrayList<>(payloads.size());
    for (Payload payload : payloads) {
      String encoded = SerContext.signatureOf(payload);
      if (!encoded.equals(signature)) {
        throw new PayloadCodecException(
            "serialization context mismatch: payload encoded as '"
                + encoded
                + "', decoded as '"
                + signature
                + "'");
      }
      result.add(payload.toBuilder().removeMetadata(SerContext.METADATA_KEY).build());
    }
    return result;
  }
}
