import { Feature } from '@temporalio/harness';
import * as proto from '@temporalio/proto';
import { patchProtobufRoot } from '@temporalio/proto/lib/patch-protobuf-root';
import { fromProto3JSON } from 'proto3-json-serializer';
import { decode } from '@temporalio/common/lib/encoding';
import * as assert from 'assert';

// Cast to `any` because the generated proto module types are missing the `lookupType` method
const patched = patchProtobufRoot(proto) as any;
const dataBlobType = patched.lookupType('temporal.api.common.v1.DataBlob');

// Inject Buffer and Uint8Array into isolate to workaround SDK bug
// TODO(antlai-temporal) Remove workaround when SDK bug is fixed
const g = globalThis as any
g.Uint8Array = g.constructor.constructor('return globalThis.Uint8Array')()
g.Buffer = g.constructor.constructor('return globalThis.Buffer')()

const expectedResult = proto.temporal.api.common.v1.DataBlob.create({
  encodingType: proto.temporal.api.enums.v1.EncodingType.ENCODING_TYPE_UNSPECIFIED,
  data: new Uint8Array([0xde, 0xad, 0xbe, 0xef]),
});

// An "echo" workflow
export async function workflow(res: proto.temporal.api.common.v1.DataBlob): Promise<proto.temporal.api.common.v1.DataBlob> {
  return res;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: {
    args: [expectedResult]
  },
  dataConverter: {
    payloadConverterPath: require.resolve('./json_protobuf_converter')
  },
  async checkResult(runner, handle) {
    // verify client result is DataBlob `0xdeadbeef`
    const result = await handle.result();
    assert.deepEqual(result.toJSON(), expectedResult.toJSON());

    // get result payload of WorkflowExecutionCompleted event from workflow history
    const payload = await runner.getWorkflowResultPayload(handle);
    assert.ok(payload);

    assert.ok(payload.metadata?.encoding);
    assert.equal(Buffer.from(payload.metadata.encoding).toString(),'json/protobuf')

    assert.ok(payload.metadata?.messageType);
    assert.equal(Buffer.from(payload.metadata.messageType).toString(), 'temporal.api.common.v1.DataBlob')

    assert.ok(payload.data)
    const resultInHistory = fromProto3JSON(dataBlobType, JSON.parse(decode(payload.data)))
    assert.ok(resultInHistory)
    assert.deepEqual(resultInHistory.toJSON(), expectedResult.toJSON());

    // get argument payload of WorkflowExecutionStarted event from workflow history
    const payloadArg = await runner.getWorkflowArgumentPayload(handle);
    assert.ok(payloadArg);

    assert.ok(payloadArg.data)
    const resultArgInHistory = fromProto3JSON(dataBlobType, JSON.parse(decode((payloadArg.data))))
    assert.ok(resultArgInHistory)
    assert.deepEqual(resultInHistory.toJSON(), resultArgInHistory.toJSON());
  },
});
