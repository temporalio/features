import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { ApplicationFailure } from '@temporalio/common';
import * as wf from '@temporalio/workflow';

// Allow 4 retries with no backoff
const activities = wf.proxyActivities<typeof activitiesImpl>({
  startToCloseTimeout: '1 minute',
  heartbeatTimeout: '5 seconds',
  // Disable retry
  retry: { maximumAttempts: 1 },
});

// Run a workflow that calls an activity that fails
export async function workflow(): Promise<void> {
  try {
    await activities.failureActivity();
  } catch (e) {
    return;
  }
}

const activitiesImpl = {
  async failureActivity() {
    let e = Error('cause error');
    e.stack = 'cause stack trace';
    let applicationError = new ApplicationFailure('main error', null, true, undefined, e);
    applicationError.stack = 'main stack trace';
    throw applicationError;
  },
};

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  async checkResult(runner, handle) {
    await handle.result();

    // get result payload of an ActivityTaskFailedEventAttributes event from workflow history
    const events = await runner.getHistoryEvents(handle);
    const completedEvent = events.find(({ activityTaskFailedEventAttributes }) => !!activityTaskFailedEventAttributes);

    const failure = completedEvent?.activityTaskFailedEventAttributes?.failure;
    assert.ok(failure);
    assert.equal('Encoded failure', failure.message);
    assert.equal('', failure.stackTrace);
    assert.equal('json/plain', failure.encodedAttributes?.metadata?.['encoding']);
    assert.equal('main error', JSON.parse(failure.encodedAttributes?.data?.toString() ?? '')['message']);
    assert.equal('main stack trace', JSON.parse(failure.encodedAttributes?.data?.toString() ?? '')['stack_trace']);
    const cause = failure.cause;
    assert.ok(cause);
    assert.equal('Encoded failure', cause.message);
    assert.equal('', cause.stackTrace);
    assert.equal('json/plain', cause.encodedAttributes?.metadata?.['encoding']);
    assert.equal('cause error', JSON.parse(cause.encodedAttributes?.data?.toString() ?? '')['message']);
    assert.equal('cause stack trace', JSON.parse(cause.encodedAttributes?.data?.toString() ?? '')['stack_trace']);
  },
  dataConverter: {
    failureConverterPath: __dirname + '/failure_converter.js',
  },
});
