import { WorkflowUpdateFailedError, WorkflowUpdateStage } from '@temporalio/client';
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
  const handler = (arg: string) => {
    state = arg;
    return 'update-result';
  };
  const validator = (arg: string) => {
    if (arg === 'invalid-arg') {
      throw new Error('Invalid Update argument');
    }
  };
  wf.setHandler(myUpdate, handler, { validator });
  await wf.condition(() => state != '');
  return state;
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    const badUpdateHandle = await handle.startUpdate(myUpdate, { args: ['invalid-arg'], waitForStage: WorkflowUpdateStage.ACCEPTED });
    try {
      await badUpdateHandle.result();
      throw 'Expected update to fail';
    } catch (err) {
      if (!(err instanceof WorkflowUpdateFailedError)) {
        throw err;
      }
    }

    const updateHandle = await handle.startUpdate(myUpdate, { args: ['update-arg'], waitForStage: WorkflowUpdateStage.ACCEPTED });
    const updateResult = await updateHandle.result();
    assert.equal(updateResult, 'update-result');
    const workflowResult = await runner.waitForRunResult(handle);
    assert.equal(workflowResult, 'update-arg');
  },
});
