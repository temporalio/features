import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { METADATA_ENCODING_KEY, Payload, PayloadCodec, ValueError } from '@temporalio/common';
import * as proto from '@temporalio/proto';
import { decode, encode } from '@temporalio/common/lib/encoding';

const toBase64 = (inArray: Uint8Array): Uint8Array => {
  const buf = Buffer.from(inArray);
  return encode(buf.toString('base64'));
};
const fromBase64 = (inArray: Uint8Array): Uint8Array => Buffer.from(decode(inArray), 'base64');

class Base64Codec implements PayloadCodec {
  async encode(payloads: Payload[]): Promise<Payload[]> {
    return payloads.map((payload) => ({
      metadata: {
        [METADATA_ENCODING_KEY]: encode(ENCODING),
      },
      data: toBase64(proto.temporal.api.common.v1.Payload.encode(payload).finish()),
    }));
  }
  async decode(payloads: Payload[]): Promise<Payload[]> {
    return payloads.map((payload) => {
      if (!payload.metadata || decode(payload.metadata[METADATA_ENCODING_KEY]) !== ENCODING) {
        return payload;
      }
      if (!payload.data) {
        throw new ValueError('Payload data is missing');
      }
      return proto.temporal.api.common.v1.Payload.decode(fromBase64(payload.data));
    });
  }
}

type resultType = {
  spec: boolean;
};

const expectedResult: resultType = { spec: true };
const ENCODING = 'my-encoding';

// An "echo" workflow
export async function workflow(res: resultType): Promise<resultType> {
  return res;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: {
    args: [expectedResult],
  },
  dataConverter: {
    payloadCodecs: [new Base64Codec()],
    // Default converter already supports JSON-serializable objects
  },
  async checkResult(runner, handle) {
    // verify client result is `{"spec": true}`
    const result = await handle.result();
    assert.deepEqual(result, expectedResult);

    // get result payload of WorkflowExecutionCompleted event from workflow history
    const payload = await runner.getWorkflowResultPayload(handle);
    assert.ok(payload);

    assert.ok(payload.metadata?.encoding);
    assert.equal(Buffer.from(payload.metadata.encoding).toString(), ENCODING);

    assert.ok(payload.data);
    const innerPayload = proto.temporal.api.common.v1.Payload.decode(fromBase64(payload.data));

    assert.ok(innerPayload.metadata?.encoding);
    assert.equal(Buffer.from(innerPayload.metadata.encoding).toString(), 'json/plain');

    const resultInHistory = JSON.parse(innerPayload.data.toString());
    assert.deepEqual(resultInHistory, expectedResult);

    // get argument payload of WorkflowExecutionStarted event from workflow history
    const payloadArg = await runner.getWorkflowArgumentPayload(handle);

    assert.ok(payloadArg?.data);
    const innerArgPayload = proto.temporal.api.common.v1.Payload.decode(fromBase64(payloadArg.data));

    assert.deepEqual(innerPayload.toJSON(), innerArgPayload.toJSON());
  },
});
