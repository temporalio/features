import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

export const finishSignal = wf.defineSignal('finish');
export const query = wf.defineQuery<string>('qq');
export const queryNum = wf.defineQuery<number>('qq');

export async function workflow(): Promise<void> {
  wf.setHandler(query, () => {
    return "hi bob"
  });
  await new Promise((resolve) => wf.setHandler(finishSignal, () => resolve(null)));
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {

    const res = await handle.query(queryNum)
    // We aren't able to verify the types line up in JS :(
    assert.equal(res, "hi bob");

    await handle.signal(finishSignal);
    await runner.waitForRunResult(handle);
  },
});
