import * as assert from 'assert';
import { randomUUID } from 'crypto';
import { ApplicationFailure } from '@temporalio/common';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

const addSignal = wf.defineSignal<[string]>('add');

export async function targetWorkflow(value: string): Promise<string[]> {
  const signals: string[] = [];
  wf.setHandler(addSignal, (signalValue) => {
    signals.push(`signal: ${signalValue}`);
  });
  await wf.condition(() => signals.length === 2);
  return [`started: ${value}`, ...signals];
}

export async function callerWorkflow(targetId: string, taskQueue: string): Promise<[string, string]> {
  const firstHandle = await wf.signalWithStartWorkflow({
    workflow: targetWorkflow,
    args: ['start-value'],
    id: targetId,
    taskQueue,
    signal: addSignal,
    signalArgs: ['signal-one'],
    idConflictPolicy: wf.WorkflowIdConflictPolicy.USE_EXISTING,
  });
  const secondHandle = await wf.signalWithStartWorkflow({
    workflow: targetWorkflow,
    args: ['unused-start-value'],
    id: targetId,
    taskQueue,
    signal: addSignal,
    signalArgs: ['signal-two'],
    idConflictPolicy: wf.WorkflowIdConflictPolicy.USE_EXISTING,
  });

  const firstRunId = firstHandle.runId;
  const secondRunId = secondHandle.runId;
  if (firstRunId === undefined) {
    throw ApplicationFailure.nonRetryable('first SignalWithStart returned no run ID');
  }
  if (secondRunId === undefined) {
    throw ApplicationFailure.nonRetryable('second SignalWithStart returned no run ID');
  }
  if (firstRunId !== secondRunId) {
    throw ApplicationFailure.nonRetryable(
      `SignalWithStart calls targeted different runs: first='${firstRunId}' second='${secondRunId}'`,
    );
  }
  return [targetId, firstRunId];
}

export const feature = new Feature({
  execute: async (runner) => {
    const targetId = `system-nexus-signal-with-start-${runner.options.taskQueue}`;
    return await runner.client.workflow.start(callerWorkflow, {
      args: [targetId, runner.options.taskQueue],
      taskQueue: runner.options.taskQueue,
      workflowId: `${runner.source.relDir}-${randomUUID()}`,
      workflowExecutionTimeout: 60000,
    });
  },
  checkResult: async (runner, handle) => {
    const [targetId, targetRunId] = await runner.waitForRunResult(handle);
    const targetHandle = runner.client.workflow.getHandle(targetId, targetRunId);
    const targetResult = await targetHandle.result();
    assert.deepEqual(targetResult, ['started: start-value', 'signal: signal-one', 'signal: signal-two']);
  },
});
