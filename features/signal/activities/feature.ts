import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import * as assert from 'assert';

const mySignal = wf.defineSignal<any[]>('mySignal');
const activityCount = 5;
const activityResult = 6;

const activitiesImpl = {
  async myActivity(): Promise<number> {
    return activityResult;
  },
};

const activities = wf.proxyActivities<typeof activitiesImpl>({
  startToCloseTimeout: '5s',
});

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  checkResult: async (_, handle) => {
    await handle.signal(mySignal);
    const result = await handle.result();
    assert.equal(result, activityResult * activityCount);
  },
});

export async function workflow(): Promise<number> {
  let total = 0;
  wf.setHandler(mySignal, async () => {
    const promises = Array.from({ length: activityCount }, activities.myActivity);
    const counts = await Promise.all(promises);
    total = counts.reduce((a, b) => a + b, 0);
  });
  await wf.condition(() => total > 0);
  return total;
}
