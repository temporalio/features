import { DefaultFailureConverter } from '@temporalio/common';

export const failureConverter = new DefaultFailureConverter({
  encodeCommonAttributes: true,
});
