import * as assert from 'assert';
import * as wf from '@temporalio/workflow';
import { ApplicationFailure } from '@temporalio/common';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const ACTIVITY_ERROR_MESSAGE = 'activity failed';
const WORKFLOW_ERROR_MESSAGE = 'workflow failed';
const ACTIVITY_ID = 'ser-ctx-activity';

const activities = wf.proxyActivities<typeof activitiesImpl>({
  activityId: ACTIVITY_ID,
  startToCloseTimeout: '10 seconds',
  retry: { maximumAttempts: 1 },
});

// Lets an activity fail and then fails itself, so that both an activity scoped
// and a workflow scoped failure conversion are recorded.
export async function workflow(): Promise<void> {
  try {
    await activities.failingActivity();
  } catch {
    throw ApplicationFailure.create({ message: WORKFLOW_ERROR_MESSAGE, type: 'WorkflowError' });
  }
  throw ApplicationFailure.create({ message: 'expected the activity to fail' });
}

const activitiesImpl = {
  async failingActivity(): Promise<void> {
    throw ApplicationFailure.create({
      message: ACTIVITY_ERROR_MESSAGE,
      type: 'ActivityError',
      nonRetryable: true,
    });
  },
};

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  dataConverter: {
    payloadCodecs: [new sercontext.SigningCodec()],
    failureConverterPath: require.resolve('../sercontext/failure_converter'),
  },
  checkResult: async (runner, handle) => {
    await assert.rejects(handle.result());

    const events = await runner.getHistoryEvents(handle);

    const activityFailed = sercontext.findEvent(
      events,
      'ActivityTaskFailed',
      (e) => !!e.activityTaskFailedEventAttributes,
    ).activityTaskFailedEventAttributes;
    assert.equal(
      activityFailed?.failure?.source,
      sercontext.activitySignature(runner.options.namespace, handle.workflowId, ACTIVITY_ID, false),
    );

    const workflowFailed = sercontext.findEvent(
      events,
      'WorkflowExecutionFailed',
      (e) => !!e.workflowExecutionFailedEventAttributes,
    ).workflowExecutionFailedEventAttributes;
    assert.equal(
      workflowFailed?.failure?.source,
      sercontext.workflowSignature(runner.options.namespace, handle.workflowId),
    );
  },
});
