package serialization_context.sercontext;

import io.temporal.api.failure.v1.Failure;
import io.temporal.common.converter.DataConverter;
import io.temporal.common.converter.FailureConverter;
import io.temporal.failure.DefaultFailureConverter;
import io.temporal.payload.context.SerializationContext;
import javax.annotation.Nonnull;

/** Records the signature of its serialization context in {@code Failure.source}. */
public class SigningFailureConverter implements FailureConverter {

  private final DefaultFailureConverter parent = new DefaultFailureConverter();
  private final String signature;

  public SigningFailureConverter() {
    this(SerContext.NO_CONTEXT);
  }

  private SigningFailureConverter(String signature) {
    this.signature = signature;
  }

  @Nonnull
  @Override
  public FailureConverter withContext(@Nonnull SerializationContext context) {
    return new SigningFailureConverter(SerContext.signature(context));
  }

  @Nonnull
  @Override
  public RuntimeException failureToException(
      @Nonnull Failure failure, @Nonnull DataConverter dataConverter) {
    return parent.failureToException(failure, dataConverter);
  }

  @Nonnull
  @Override
  public Failure exceptionToFailure(
      @Nonnull Throwable throwable, @Nonnull DataConverter dataConverter) {
    Failure failure = parent.exceptionToFailure(throwable, dataConverter);
    // A failure that already travelled the wire keeps the source it was created with, so only a
    // freshly built one is stamped.
    if (SerContext.DEFAULT_FAILURE_SOURCE.equals(failure.getSource())) {
      return failure.toBuilder().setSource(signature).build();
    }
    return failure;
  }
}
