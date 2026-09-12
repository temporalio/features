import * as assert from 'assert';
import { CompleteAsyncError, Context } from '@temporalio/activity';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const ACTIVITY_RESULT = 'completed-out-of-band';
const HEARTBEAT_DATA = 'beat';
const ACTIVITY_ID = 'ser-ctx-activity';

const activities = wf.proxyActivities<typeof activitiesImpl>({
  activityId: ACTIVITY_ID,
  startToCloseTimeout: '1 minute',
  heartbeatTimeout: '30 seconds',
});

export async function workflow(): Promise<string> {
  return activities.pendingActivity();
}

// What the activity worker saw, used by the completing client to reconstruct
// the same activity serialization context.
let taskToken: Uint8Array | undefined;
let workflowId: string | undefined;

const activitiesImpl = {
  async pendingActivity(): Promise<string> {
    const info = Context.current().info;
    taskToken = info.taskToken;
    workflowId = info.workflowExecution?.workflowId;
    throw new CompleteAsyncError();
  },
};

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  execute: async (runner) => {
    const handle = await runner.executeSingleParameterlessWorkflow();

    for (let i = 0; i < 300 && taskToken === undefined; i++) {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    assert.ok(taskToken, 'activity was never started');
    assert.ok(workflowId);

    // A task token carries no activity metadata, so the context has to be
    // supplied. By-ID operations infer it from the IDs instead.
    const serializationContext = {
      type: 'activity' as const,
      namespace: runner.options.namespace,
      workflowId,
      activityId: ACTIVITY_ID,
      isLocal: false,
    };
    await runner.client.activity.heartbeat(taskToken, HEARTBEAT_DATA, { serializationContext });
    await runner.client.activity.complete(taskToken, ACTIVITY_RESULT, { serializationContext });
    return handle;
  },
  checkResult: async (runner, handle) => {
    assert.equal(await handle.result(), ACTIVITY_RESULT);

    const events = await runner.getHistoryEvents(handle);
    const completed = sercontext.findEvent(
      events,
      'ActivityTaskCompleted',
      (e) => !!e.activityTaskCompletedEventAttributes,
    ).activityTaskCompletedEventAttributes;
    assert.equal(
      sercontext.firstSignature(completed?.result),
      sercontext.activitySignature(runner.options.namespace, handle.workflowId, ACTIVITY_ID, false),
    );
  },
});
