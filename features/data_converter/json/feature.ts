import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

type resultType = {
  spec: boolean;
};

const expectedResult: resultType = { spec: true };

// An "echo" workflow
export async function workflow(res: resultType): Promise<resultType> {
  return res;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: {
    args: [expectedResult],
  },
  // Default converter already supports JSON-serializable objects
  async checkResult(runner, handle) {
    // verify client result is `{"spec": true}`
    const result = await handle.result();
    assert.deepEqual(result, expectedResult);

    // get result payload of WorkflowExecutionCompleted event from workflow history
    const payload = await runner.getWorkflowResultPayload(handle);
    assert.ok(payload);

    assert.ok(payload.metadata?.encoding);
    assert.equal(Buffer.from(payload.metadata.encoding).toString(), 'json/plain');

    assert.ok(payload.data);
    const resultInHistory = JSON.parse(payload.data.toString());
    assert.deepEqual(resultInHistory, expectedResult);

    // get argument payload of WorkflowExecutionStarted event from workflow history
    const payloadArg = await runner.getWorkflowArgumentPayload(handle);

    assert.ok(payloadArg?.data);
    const resultArgInHistory = JSON.parse(payloadArg.data.toString());
    assert.deepEqual(resultInHistory, resultArgInHistory);
  },
});
