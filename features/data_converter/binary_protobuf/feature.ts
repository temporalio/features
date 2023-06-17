import { Feature } from '@temporalio/harness';
import * as proto from '@temporalio/proto';
import * as assert from 'assert';

const expectedResult = proto.temporal.api.common.v1.DataBlob.create({
  encodingType: proto.temporal.api.enums.v1.EncodingType.ENCODING_TYPE_UNSPECIFIED,
  data: new Uint8Array([0xde, 0xad, 0xbe, 0xef]),
});

const expectedResultFlat = proto.temporal.api.common.v1.DataBlob.encode(expectedResult).finish()
console.log(expectedResultFlat)
// An "echo" workflow
export async function workflow(resBytes: Uint8Array): Promise<proto.temporal.api.common.v1.DataBlob> {
  console.log(Object.prototype.toString.call(resBytes))
  console.log(resBytes instanceof Uint8Array)
  const otherRes = new Uint8Array(resBytes)
  const res = proto.temporal.api.common.v1.DataBlob.decode(otherRes);
  return res;
}

/*
export async function workflow(): Promise<proto.temporal.api.common.v1.DataBlob> {
  return expectedResult;
}
*/

export const feature = new Feature({
  workflow,
  workflowStartOptions: {
    args: [proto.temporal.api.common.v1.DataBlob.encode(expectedResult).finish()]
  },
  dataConverter: {
    payloadConverterPath: require.resolve('./binary_protobuf_converter')
  },
  async checkResult(runner, handle) {
    // verify client result is binary `0xdeadbeef`
    const result = await handle.result();
    console.log(result)
    assert.deepEqual(result.toJSON(), expectedResult.toJSON());


    // get result payload of WorkflowExecutionCompleted event from workflow history
    const payload = await runner.getWorkflowResultPayload(handle);
    assert.ok(payload);

    assert.ok(payload.metadata?.encoding);
    assert.equal(Buffer.from(payload.metadata.encoding).toString(),'binary/protobuf')

    assert.ok(payload.metadata?.messageType);
    assert.equal(Buffer.from(payload.metadata.messageType).toString(),
                 'temporal.api.common.v1.DataBlob')

    assert.ok(payload.data)
    const resultInHistory = proto.temporal.api.common.v1.DataBlob.decode(payload.data)

    console.log(resultInHistory)
    assert.deepEqual(resultInHistory.toJSON(), expectedResult.toJSON());


    /*
    // load JSON payload from `./payload.json` and compare it to result payload
    assert.deepEqual(JSONToPayload(expectedPayload), payload);
    */
  },
});
