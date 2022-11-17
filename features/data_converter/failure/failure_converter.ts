import { ActivityFailure, DefaultPayloadConverter, DefaultFailureConverter } from '@temporalio/common';

export const failureConverter = new DefaultFailureConverter({
  payloadConverter: new DefaultPayloadConverter(),
  encodeCommonAttributes: true,
});
