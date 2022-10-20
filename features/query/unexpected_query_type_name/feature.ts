import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { QueryNotRegisteredError } from '@temporalio/client';

export const finishSignal = wf.defineSignal('finish');
export const query = wf.defineQuery<string>('nonexistent');

export async function workflow(): Promise<void> {
  await new Promise((resolve) => wf.setHandler(finishSignal, () => resolve(null)));
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    await assert.rejects(async () => {
      await handle.query(query);
    }, QueryNotRegisteredError);

    await handle.signal(finishSignal);
    await runner.waitForRunResult(handle);
  },
});
