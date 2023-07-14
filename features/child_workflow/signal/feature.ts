import { randomUUID } from 'crypto';
import * as assert from 'assert';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

const unblockMessage = 'unblock';

// A workflow that starts a child workflow, unblocks it, and returns the result
// of the child workflow.
export async function workflow(): Promise<string> {
  const childHandle = await wf.startChild(childWorkflow);
  await childHandle.signal(unblock, unblockMessage);
  return await childHandle.result();
}

const unblock = wf.defineSignal<[string]>('unblock');

// A workflow that waits for a signal and returns the data received.
export async function childWorkflow(): Promise<string> {
  let unblockMessage = '';
  wf.setHandler(unblock, (message: string) => {
    unblockMessage = message;
  });
  await wf.condition(() => unblockMessage !== '');
  return unblockMessage;
}

export const feature = new Feature({
  workflow,
  async execute(runner) {
    return await runner.client.start(workflow, {
      taskQueue: runner.options.taskQueue,
      workflowId: `${runner.source.relDir}-${randomUUID()}`,
      workflowExecutionTimeout: 60000,
      ...(runner.feature.options.workflowStartOptions ?? {}),
    });
  },
  async checkResult(runner, handle) {
    const result = await handle.result();
    assert.equal(result, unblockMessage);
  },
});
