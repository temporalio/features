import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import * as assert from 'assert';

const mySignal = wf.defineSignal<string[]>('mySignal');
const signalData = 'signal-data';

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    await handle.signal(mySignal, signalData);
    const workflowResult = await runner.waitForRunResult(handle);
    assert.equal(workflowResult, signalData);
  },
});

export async function workflow(): Promise<string> {
  let result = '';
  wf.setHandler(mySignal, (str: string) => {
    result = str;
  });
  await wf.condition(() => result != '');
  return result;
}
