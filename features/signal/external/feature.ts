import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import * as assert from 'assert';

const signalData = 'Signaled!';
const externalSignal = wf.defineSignal<[string]>('externalSignal');

// A workflow that receives a signal and immediately returns with the value of said signal.
export async function workflow(): Promise<string> {
  let result = '';
  wf.setHandler(externalSignal, (str: string) => {
    result = str;
  });
  await wf.condition(() => {
    return result != '';
  });
  return result;
}

export const feature =
  !wf.inWorkflowContext() &&
  new Feature({
    workflow,
    execute: async (runner) => {
      const handle = await runner.executeSingleParameterlessWorkflow();
      await handle.signal(externalSignal, signalData);
      return handle;
    },
    checkResult: async (runner, run) => {
      const result = await runner.waitForRunResult(run);
      assert.equal(result, signalData);
    },
  });
