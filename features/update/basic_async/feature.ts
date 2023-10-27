import { WorkflowUpdateFailedError } from '@temporalio/client';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import * as assert from 'assert';

const WORKFLOW_INITIAL_STATE = '';
const UPDATE_ARG = 'update-arg';
const BAD_UPDATE_ARG = 'reject-me';
const UPDATE_RESULT = 'update-result';
const UPDATE_RESULT_NOT_YET_RECEIVED = 'update-result-not-yet-received';

const myUpdate = wf.defineUpdate<string, [string]>('myUpdate');

/**
 * A workflow with a signal and signal validator. If accepted, the signal makes
 * a change to workflow state. The workflow does not terminate until such a
 * change occurs.
 */
export async function workflow(): Promise<string> {
  let state = WORKFLOW_INITIAL_STATE;
  const handler = (arg: string) => {
    state = arg;
    return UPDATE_RESULT;
  };
  const validator = (arg: string) => {
    if (arg == BAD_UPDATE_ARG) {
      throw new Error('Invalid Update argument');
    }
  };
  wf.setHandler(myUpdate, handler, { validator });
  await wf.condition(() => state != WORKFLOW_INITIAL_STATE);
  return state;
}

export const feature = new Feature({
  workflow,
  checkResult: async (runner, handle) => {
    const badUpdateHandle = await handle.startUpdate(myUpdate, { args: [BAD_UPDATE_ARG] });
    let badUpdateResult = UPDATE_RESULT_NOT_YET_RECEIVED;
    try {
      badUpdateResult = await badUpdateHandle.result();
    } catch (err) {
      if (!(err instanceof WorkflowUpdateFailedError)) {
        throw err;
      }
    }
    assert.equal(badUpdateResult, UPDATE_RESULT_NOT_YET_RECEIVED);

    const updateHandle = await handle.startUpdate(myUpdate, { args: [UPDATE_ARG] });
    const updateResult = await updateHandle.result();
    assert.equal(updateResult, UPDATE_RESULT);
    const workflowResult = await runner.waitForRunResult(handle);
    assert.equal(workflowResult, UPDATE_ARG);
  },
});
