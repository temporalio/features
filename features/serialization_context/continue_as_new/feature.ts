import * as assert from 'assert';
import * as wf from '@temporalio/workflow';
import * as proto from '@temporalio/proto';
import { Feature, Runner } from '@temporalio/harness';
import * as sercontext from '../sercontext/sercontext';

const FINAL_RESULT = 'done';

export async function workflow(remaining: number): Promise<string> {
  if (remaining > 0) {
    await wf.continueAsNew<typeof workflow>(remaining - 1);
  }
  return FINAL_RESULT;
}

// The harness helper always reads the latest run, and this feature needs a
// specific one.
async function runHistoryEvents(
  runner: Runner<any, any>,
  workflowId: string,
  runId?: string,
): Promise<proto.temporal.api.history.v1.IHistoryEvent[]> {
  const events = Array<proto.temporal.api.history.v1.IHistoryEvent>();
  let nextPageToken: Uint8Array | undefined = undefined;
  for (;;) {
    const response: proto.temporal.api.workflowservice.v1.GetWorkflowExecutionHistoryResponse =
      await runner.client.connection.workflowService.getWorkflowExecutionHistory({
        nextPageToken,
        namespace: runner.options.namespace,
        execution: { workflowId, runId },
      });
    events.push(...(response.history?.events ?? []));
    if (response.nextPageToken == null || response.nextPageToken.length === 0) break;
    nextPageToken = response.nextPageToken;
  }
  return events;
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: { args: [1] },
  dataConverter: { payloadCodecs: [new sercontext.SigningCodec()] },
  checkResult: async (runner, handle) => {
    assert.equal(await handle.result(), FINAL_RESULT);

    // Continue-as-new keeps the workflow ID, so both runs share the context.
    const expected = sercontext.workflowSignature(runner.options.namespace, handle.workflowId);

    const firstRunEvents = await runHistoryEvents(runner, handle.workflowId, handle.firstExecutionRunId);
    const continued = sercontext.findEvent(
      firstRunEvents,
      'WorkflowExecutionContinuedAsNew',
      (e) => !!e.workflowExecutionContinuedAsNewEventAttributes,
    ).workflowExecutionContinuedAsNewEventAttributes;
    assert.equal(sercontext.firstSignature(continued?.input), expected);

    const lastRunEvents = await runHistoryEvents(runner, handle.workflowId);
    const started = sercontext.findEvent(
      lastRunEvents,
      'WorkflowExecutionStarted',
      (e) => !!e.workflowExecutionStartedEventAttributes,
    ).workflowExecutionStartedEventAttributes;
    assert.equal(sercontext.firstSignature(started?.input), expected);

    const completed = sercontext.findEvent(
      lastRunEvents,
      'WorkflowExecutionCompleted',
      (e) => !!e.workflowExecutionCompletedEventAttributes,
    ).workflowExecutionCompletedEventAttributes;
    assert.equal(sercontext.firstSignature(completed?.result), expected);
  },
});
