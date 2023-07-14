import { JSONToPayload } from '@temporalio/common/lib/proto-utils';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import expectedPayload from './payload.json';

const deadbeef = new Uint8Array([0xde, 0xad, 0xbe, 0xef]);

// run an echo workflow that returns binary value `0xdeadbeef`
export async function workflow(res: Uint8Array): Promise<Uint8Array> {
  return res;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: {
    args: [deadbeef],
  },
  async checkResult(runner, handle) {
    // verify client result is binary `0xdeadbeef`
    const result = await handle.result();
    assert.deepEqual(result, deadbeef);

    // get result payload of WorkflowExecutionCompleted event from workflow history
    const payload = await runner.getWorkflowResultPayload(handle);
    assert.ok(payload);

    // load JSON payload from `./payload.json` and compare it to result payload
    assert.deepEqual(JSONToPayload(expectedPayload), payload);

    const payloadArg = await runner.getWorkflowArgumentPayload(handle);
    assert.ok(payloadArg);

    assert.deepEqual(payload, payloadArg);
  },
});
