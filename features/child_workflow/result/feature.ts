import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

const ChildWorkflowInput = 'test';

export async function workflow(): Promise<string> {
  return wf.executeChild('childWorkflow', {
    args: [ChildWorkflowInput],
  });
}

export async function childWorkflow(input: string): Promise<string> {
  return input;
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    const result = await runner.waitForRunResult(handle);
    assert.equal(ChildWorkflowInput, result);
  },
});
