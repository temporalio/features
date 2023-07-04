package data_converter.codec;

import com.google.protobuf.ByteString;
import io.temporal.api.common.v1.Payload;
import io.temporal.common.converter.EncodingKeys;
import io.temporal.payload.codec.PayloadCodec;
import io.temporal.payload.codec.PayloadCodecException;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.List;
import java.util.stream.Collectors;

// Adapted from samples-java, encryptedpayloads/CryptCodec.java
class Base64Codec implements PayloadCodec {

  public static final ByteString METADATA_ENCODING =
      ByteString.copyFrom("my-encoding", StandardCharsets.UTF_8);

  @Override
  public List<Payload> encode(List<Payload> payloads) {
    return payloads.stream().map(this::encodePayload).collect(Collectors.toList());
  }

  @Override
  public List<Payload> decode(List<Payload> payloads) {
    return payloads.stream().map(this::decodePayload).collect(Collectors.toList());
  }

  private Payload encodePayload(Payload payload) {
    byte[] encodedData;
    try {
      encodedData = Base64.getEncoder().encodeToString(payload.toByteArray()).getBytes();
    } catch (Throwable e) {
      throw new PayloadCodecException(e);
    }

    return Payload.newBuilder()
        .putMetadata(EncodingKeys.METADATA_ENCODING_KEY, METADATA_ENCODING)
        .setData(ByteString.copyFrom(encodedData))
        .build();
  }

  private Payload decodePayload(Payload payload) {
    if (METADATA_ENCODING.equals(
        payload.getMetadataOrDefault(EncodingKeys.METADATA_ENCODING_KEY, null))) {
      try {
        byte[] plainData = Base64.getDecoder().decode(new String(payload.getData().toByteArray()));
        Payload decodedPayload = Payload.parseFrom(plainData);
        return decodedPayload;
      } catch (Throwable e) {
        throw new PayloadCodecException(e);
      }
    } else {
      return payload;
    }
  }
}
