import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import * as wf from '@temporalio/workflow';

const increment = wf.defineUpdate<void>('increment');
const getCount = wf.defineQuery<number>('getCount');
const exit = wf.defineUpdate<void>('exit');

export const feature = new Feature({
  workflow,
  checkResult: async (_, handle) => {
    const updateId = 'myUpdateId';
    await handle.executeUpdate(increment, { updateId });
    assert.equal(await handle.query(getCount), 1);
    await handle.executeUpdate(increment, { updateId });
    assert.equal(await handle.query(getCount), 1);
    await handle.executeUpdate(exit, {});
    const count = await handle.result();
    assert.equal(count, 1);
  },
});

export async function workflow(): Promise<number> {
  let count = 0;
  let readyToExitWorkflow = false;
  wf.setHandler(increment, () => {
    count += 1;
  });
  wf.setHandler(exit, () => {
    readyToExitWorkflow = true;
  });
  wf.setHandler(getCount, () => count);
  await wf.condition(() => readyToExitWorkflow);
  return count;
}
