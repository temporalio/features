import { WorkflowUpdateFailedError } from '@temporalio/client';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import * as assert from 'assert';

const myUpdate = wf.defineUpdate<string, [string]>('myUpdate');

/**
 * A workflow with an update and an update validator. If accepted, the update
 * makes a change to workflow state. The workflow does not terminate until such
 * a change occurs.
 */
export async function workflow(): Promise<string> {
  let state = '';
  const validator = (arg: string) => {
    if (arg == 'invalid-arg') {
      throw new Error('Invalid Update argument');
    }
  };
  const handler = async (arg: string) => {
    state = arg;
    return 'update-result';
  };
  wf.setHandler(myUpdate, handler, { validator });
  await wf.condition(() => state != '');
  return state;
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    try {
      await handle.executeUpdate(myUpdate, { args: ['invalid-arg'] });
      throw 'Expected update to fail';
    } catch (err) {
      if (!(err instanceof WorkflowUpdateFailedError)) {
        throw err;
      }
    }

    const updateResult = await handle.executeUpdate(myUpdate, { args: ['update-arg'] });
    assert.equal(updateResult, 'update-result');
    const workflowResult = await runner.waitForRunResult(handle);
    assert.equal(workflowResult, 'update-arg');
  },
});
