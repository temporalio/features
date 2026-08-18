import {
  DefaultFailureConverter,
  FailureConverter,
  PayloadConverter,
  ProtoFailure,
  SerializationContext,
} from '@temporalio/common';
import { signatureOf } from './sercontext';

const DEFAULT_FAILURE_SOURCE = 'TypeScriptSDK';

/** Records the signature of its serialization context in `Failure.source`. */
class SigningFailureConverter implements FailureConverter {
  private readonly parent = new DefaultFailureConverter();

  errorToFailure(err: unknown, payloadConverter: PayloadConverter, context?: SerializationContext): ProtoFailure {
    const failure = this.parent.errorToFailure(err, payloadConverter, context);
    // A failure that already travelled the wire keeps the source it was created
    // with, so only a freshly built one is stamped.
    if (failure.source === DEFAULT_FAILURE_SOURCE) {
      failure.source = signatureOf(context);
    }
    return failure;
  }

  failureToError(failure: ProtoFailure, payloadConverter: PayloadConverter, context?: SerializationContext) {
    return this.parent.failureToError(failure, payloadConverter, context);
  }
}

export const failureConverter = new SigningFailureConverter();
