import * as assert from 'assert';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const SIGNAL_DATA = 'signaled';

const externalSignal = wf.defineSignal<[string]>('external');

// Signals another running workflow. The payload is serialized with the target's
// workflow ID, not this workflow's own ID.
export async function workflow(targetId: string): Promise<string> {
  await wf.getExternalWorkflowHandle(targetId).signal(externalSignal, SIGNAL_DATA);
  return targetId;
}

export async function receiver(): Promise<string> {
  let received: string | undefined;
  wf.setHandler(externalSignal, (data) => {
    received = data;
  });
  await wf.condition(() => received !== undefined);
  return received as string;
}

function receiverId(taskQueue: string): string {
  return `${taskQueue}-receiver`;
}

export const feature = new Feature({
  workflow,
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  execute: async (runner) => {
    const receiverHandle = await runner.client.workflow.start(receiver, {
      taskQueue: runner.options.taskQueue,
      workflowId: receiverId(runner.options.taskQueue),
      workflowExecutionTimeout: 60000,
    });

    const handle = await runner.client.workflow.start(workflow, {
      taskQueue: runner.options.taskQueue,
      workflowId: `${runner.options.taskQueue}-wf`,
      workflowExecutionTimeout: 60000,
      args: [receiverHandle.workflowId],
    });

    assert.equal(await receiverHandle.result(), SIGNAL_DATA);
    return handle;
  },
  checkResult: async (runner, handle) => {
    const target = receiverId(runner.options.taskQueue);
    assert.equal(await handle.result(), target);

    const expected = sercontext.workflowSignature(runner.options.namespace, target);
    assert.notEqual(expected, sercontext.workflowSignature(runner.options.namespace, handle.workflowId));

    const senderEvents = await runner.getHistoryEvents(handle);
    const initiated = sercontext.findEvent(
      senderEvents,
      'SignalExternalWorkflowExecutionInitiated',
      (e) => !!e.signalExternalWorkflowExecutionInitiatedEventAttributes,
    ).signalExternalWorkflowExecutionInitiatedEventAttributes;
    assert.equal(sercontext.firstSignature(initiated?.input), expected);

    const receiverEvents = await runner.getHistoryEvents(runner.client.workflow.getHandle(target));
    const signaled = sercontext.findEvent(
      receiverEvents,
      'WorkflowExecutionSignaled',
      (e) => !!e.workflowExecutionSignaledEventAttributes,
    ).workflowExecutionSignaledEventAttributes;
    assert.equal(sercontext.firstSignature(signaled?.input), expected);
  },
});
