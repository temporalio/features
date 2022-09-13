import { JSONToPayload } from '@temporalio/common/lib/proto-utils';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { ApplicationFailure } from '@temporalio/common';
import expectedPayload from './payload.json';
import * as wf from '@temporalio/workflow';

const activitiesImpl = {
  async activity(input: any): Promise<void> {
    if (input != undefined) {
      throw ApplicationFailure.nonRetryable('Activity input should be undefined', 'BadResult');
    }
  },
};

const act = wf.proxyActivities<typeof activitiesImpl>({
  startToCloseTimeout: '1 minute',
});

// run a workflow that calls an activity with a undefined parameter.
export async function workflow(): Promise<void> {
  await act.activity(undefined);
}

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  async checkResult(runner, handle) {
    // verify client result is undefined
    const result = await handle.result();
    assert.strictEqual(result, undefined);

    // get result payload of ActivityTaskScheduled event from workflow history
    const events = await runner.getHistoryEvents(handle);
    const completedEvent = events.find(
      ({ activityTaskScheduledEventAttributes }) => !!activityTaskScheduledEventAttributes
    );

    const payload = completedEvent?.activityTaskScheduledEventAttributes?.input?.payloads?.[0];
    assert.ok(payload);
    // load JSON payload from `./payload.json` and compare it to result payload
    assert.deepEqual(JSONToPayload(expectedPayload), payload);
  },
});
