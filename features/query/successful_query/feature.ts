import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

export const signal = wf.defineSignal('inc-counter');
export const finishSignal = wf.defineSignal('finish');
export const query = wf.defineQuery<number>('get-counter');

export async function workflow(): Promise<void> {
  let counter = 0;
  wf.setHandler(signal, () => {
    counter = counter + 1;
  });
  wf.setHandler(query, () => {
    return counter;
  });

  await new Promise((resolve) => wf.setHandler(finishSignal, () => resolve(null)));
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    const q1 = await handle.query(query);
    assert.equal(q1, 0);
    await handle.signal(signal);
    const q2 = await handle.query(query);
    assert.equal(q2, 1);
    await handle.signal(signal);
    await handle.signal(signal);
    await handle.signal(signal);
    const q3 = await handle.query(query);
    assert.equal(q3, 4);
    await handle.signal(finishSignal);
    await runner.waitForRunResult(handle);
  },
});
