import * as assert from 'assert';
import { Context } from '@temporalio/activity';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const WORKFLOW_INPUT = 'hello';
const HEARTBEAT_DATA = 'beat';
const ACTIVITY_ID = 'ser-ctx-activity';

const activities = wf.proxyActivities<typeof activitiesImpl>({
  activityId: ACTIVITY_ID,
  startToCloseTimeout: '10 seconds',
  heartbeatTimeout: '5 seconds',
  retry: { initialInterval: '1 millisecond', maximumAttempts: 2 },
});

export async function workflow(input: string): Promise<string> {
  return activities.activityWithHeartbeat(input);
}

const activitiesImpl = {
  // Fails its first attempt so the second one has to decode the heartbeat
  // details recorded by the first.
  async activityWithHeartbeat(input: string): Promise<string> {
    const ctx = Context.current();
    if (ctx.info.attempt === 1) {
      ctx.heartbeat(HEARTBEAT_DATA);
      throw new Error('retrying to read back heartbeat details');
    }
    return `${input}|${ctx.info.heartbeatDetails}`;
  },
};

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  workflowStartOptions: { args: [WORKFLOW_INPUT] },
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  checkResult: async (runner, handle) => {
    assert.equal(await handle.result(), `${WORKFLOW_INPUT}|${HEARTBEAT_DATA}`);

    const events = await runner.getHistoryEvents(handle);
    const expected = sercontext.activitySignature(runner.options.namespace, handle.workflowId, ACTIVITY_ID, false);

    const scheduled = sercontext.findEvent(
      events,
      'ActivityTaskScheduled',
      (e) => !!e.activityTaskScheduledEventAttributes,
    ).activityTaskScheduledEventAttributes;
    assert.equal(scheduled?.activityId, ACTIVITY_ID);
    assert.equal(sercontext.firstSignature(scheduled?.input), expected);

    const completed = sercontext.findEvent(
      events,
      'ActivityTaskCompleted',
      (e) => !!e.activityTaskCompletedEventAttributes,
    ).activityTaskCompletedEventAttributes;
    assert.equal(sercontext.firstSignature(completed?.result), expected);

    const workflowCompleted = sercontext.findEvent(
      events,
      'WorkflowExecutionCompleted',
      (e) => !!e.workflowExecutionCompletedEventAttributes,
    ).workflowExecutionCompletedEventAttributes;
    assert.equal(
      sercontext.firstSignature(workflowCompleted?.result),
      sercontext.workflowSignature(runner.options.namespace, handle.workflowId),
    );
  },
});
