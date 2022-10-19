import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

export const finishSignal = wf.defineSignal('finish');
const queryName = 'myq';
export const query = wf.defineQuery<string, [number]>(queryName);
export const queryWrongType = wf.defineQuery<string, [boolean]>(queryName);
export const queryExtraArg = wf.defineQuery<string, [number, boolean]>(queryName);
export const queryNotEnoughArg = wf.defineQuery<string>(queryName);

export async function workflow(): Promise<void> {
  wf.setHandler(query, (arg: number) => {
    return queryImpl(arg)
  });

  await new Promise((resolve) => wf.setHandler(finishSignal, () => resolve(null)));
}

function queryImpl(...args: any[]) {
  return `query yo ${args.join(',')}`
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    // Typescript doesn't reject anything.

    const q1 = await handle.query(queryWrongType, true);
    assert.equal(q1, queryImpl(true))

    // It does, however, silently drop the extra argument
    const q2 = await handle.query(queryExtraArg, 123, true);
    assert.equal(q2, queryImpl(123))

    const q3 = await handle.query(queryNotEnoughArg);
    assert.equal(q3, queryImpl())

    await handle.signal(finishSignal);
    await runner.waitForRunResult(handle);
  },
});
