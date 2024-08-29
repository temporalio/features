import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import ms from 'ms';
import { Duration, StringValue } from '@temporalio/common';
import { WorkflowUpdateStage } from '@temporalio/client';

const myUpdate = wf.defineUpdate<void, [Duration, boolean]>('myUpdate');
const requestedSleep = '2s';

export const feature = new Feature({
  workflow,
  checkResult: async (_, handle) => {
    const timeToAccept = await time(handle.startUpdate(myUpdate, { args: [requestedSleep, false], waitForStage: WorkflowUpdateStage.ACCEPTED }));
    const timeToComplete = await time(handle.executeUpdate(myUpdate, { args: [requestedSleep, false] }));
    assert.equal(
      ms(timeToAccept) < ms(requestedSleep),
      true,
      `Expected timeToAccept (${timeToAccept}) < requestedSleep (${requestedSleep})`
    );
    assert.equal(
      ms(timeToComplete) >= ms(requestedSleep),
      true,
      `Expected timeToComplete (${timeToComplete}) >= requestedSleep (${requestedSleep})`
    );
    await handle.executeUpdate(myUpdate, { args: [0, true] });
    await handle.result();
  },
});

async function time(promise: Promise<any>): Promise<StringValue> {
  const t0 = process.hrtime.bigint();
  await promise;
  const t1 = process.hrtime.bigint();
  const millis = Number((t1 - t0) / BigInt('1000000'));
  return `${millis}ms`;
}

export async function workflow(): Promise<void> {
  let readyToExitWorkflow = false;
  wf.setHandler(myUpdate, async (requestedSleep: Duration, exitWorkflow: boolean) => {
    await wf.sleep(requestedSleep);
    readyToExitWorkflow = exitWorkflow;
  });
  await wf.condition(() => readyToExitWorkflow);
}
