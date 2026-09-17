import * as assert from 'assert';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const WORKFLOW_INPUT = 'hello';
const CHILD_ID_SUFFIX = '_child';
const CHILD_RESULT_TAG = '|child';

export async function workflow(input: string): Promise<string> {
  return wf.executeChild(childWorkflow, {
    workflowId: wf.workflowInfo().workflowId + CHILD_ID_SUFFIX,
    args: [input],
  });
}

export async function childWorkflow(input: string): Promise<string> {
  return input + CHILD_RESULT_TAG;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: { args: [WORKFLOW_INPUT] },
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  checkResult: async (runner, handle) => {
    assert.equal(await handle.result(), WORKFLOW_INPUT + CHILD_RESULT_TAG);

    const childId = handle.workflowId + CHILD_ID_SUFFIX;
    // The child's payloads carry the child's own workflow ID, not the parent's.
    const expected = sercontext.workflowSignature(runner.options.namespace, childId);
    assert.notEqual(expected, sercontext.workflowSignature(runner.options.namespace, handle.workflowId));

    const parentEvents = await runner.getHistoryEvents(handle);
    const initiated = sercontext.findEvent(
      parentEvents,
      'StartChildWorkflowExecutionInitiated',
      (e) => !!e.startChildWorkflowExecutionInitiatedEventAttributes,
    ).startChildWorkflowExecutionInitiatedEventAttributes;
    assert.equal(sercontext.firstSignature(initiated?.input), expected);

    const childCompleted = sercontext.findEvent(
      parentEvents,
      'ChildWorkflowExecutionCompleted',
      (e) => !!e.childWorkflowExecutionCompletedEventAttributes,
    ).childWorkflowExecutionCompletedEventAttributes;
    assert.equal(sercontext.firstSignature(childCompleted?.result), expected);

    const childEvents = await runner.getHistoryEvents(runner.client.workflow.getHandle(childId));
    const childStarted = sercontext.findEvent(
      childEvents,
      'WorkflowExecutionStarted',
      (e) => !!e.workflowExecutionStartedEventAttributes,
    ).workflowExecutionStartedEventAttributes;
    assert.equal(sercontext.firstSignature(childStarted?.input), expected);
  },
});
