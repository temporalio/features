import * as assert from 'assert';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const WORKFLOW_INPUT = 'input';
const MEMO_KEY = 'ser-ctx-memo';
const MEMO_VALUE = 'memo';
const QUERY_ARG = 'query-';
const UPDATE_ARG = '-update';
const SIGNAL_DATA = 'signal';

const appendSignal = wf.defineSignal<[string]>('append');
const prefixedQuery = wf.defineQuery<string, [string]>('prefixed');
const suffixedUpdate = wf.defineUpdate<string, [string]>('suffixed');

// Exercises every workflow scoped payload: input and result, a memo, a signal,
// a query and an update.
export async function workflow(input: string): Promise<string> {
  let signaled: string | undefined;
  wf.setHandler(appendSignal, (data) => {
    signaled = data;
  });
  wf.setHandler(prefixedQuery, (prefix) => prefix + input);
  wf.setHandler(suffixedUpdate, (suffix) => input + suffix);

  await wf.condition(() => signaled !== undefined);
  return `${input}|${signaled}`;
}

export const feature = new Feature({
  workflow,
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  execute: async (runner) => {
    const handle = await runner.client.workflow.start(workflow, {
      taskQueue: runner.options.taskQueue,
      workflowId: `${runner.options.taskQueue}-wf`,
      workflowExecutionTimeout: 60000,
      args: [WORKFLOW_INPUT],
      memo: { [MEMO_KEY]: MEMO_VALUE },
    });

    assert.equal(await handle.query(prefixedQuery, QUERY_ARG), QUERY_ARG + WORKFLOW_INPUT);
    assert.equal(await handle.executeUpdate(suffixedUpdate, { args: [UPDATE_ARG] }), WORKFLOW_INPUT + UPDATE_ARG);
    await handle.signal(appendSignal, SIGNAL_DATA);
    return handle;
  },
  checkResult: async (runner, handle) => {
    assert.equal(await handle.result(), `${WORKFLOW_INPUT}|${SIGNAL_DATA}`);

    const events = await runner.getHistoryEvents(handle);
    const expected = sercontext.workflowSignature(runner.options.namespace, handle.workflowId);

    const started = sercontext.findEvent(
      events,
      'WorkflowExecutionStarted',
      (e) => !!e.workflowExecutionStartedEventAttributes,
    ).workflowExecutionStartedEventAttributes;
    assert.equal(sercontext.firstSignature(started?.input), expected);
    assert.equal(sercontext.payloadSignature(started?.memo?.fields?.[MEMO_KEY]), expected);

    const completed = sercontext.findEvent(
      events,
      'WorkflowExecutionCompleted',
      (e) => !!e.workflowExecutionCompletedEventAttributes,
    ).workflowExecutionCompletedEventAttributes;
    assert.equal(sercontext.firstSignature(completed?.result), expected);

    const signaled = sercontext.findEvent(
      events,
      'WorkflowExecutionSignaled',
      (e) => !!e.workflowExecutionSignaledEventAttributes,
    ).workflowExecutionSignaledEventAttributes;
    assert.equal(sercontext.firstSignature(signaled?.input), expected);

    const accepted = sercontext.findEvent(
      events,
      'WorkflowExecutionUpdateAccepted',
      (e) => !!e.workflowExecutionUpdateAcceptedEventAttributes,
    ).workflowExecutionUpdateAcceptedEventAttributes;
    assert.equal(sercontext.firstSignature(accepted?.acceptedRequest?.input?.args), expected);

    const updateCompleted = sercontext.findEvent(
      events,
      'WorkflowExecutionUpdateCompleted',
      (e) => !!e.workflowExecutionUpdateCompletedEventAttributes,
    ).workflowExecutionUpdateCompletedEventAttributes;
    assert.equal(sercontext.firstSignature(updateCompleted?.outcome?.success), expected);
  },
});
