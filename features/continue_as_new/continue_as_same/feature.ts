import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

import * as assert from 'assert';

const InputData = 'InputData';
const MemoKey = 'MemoKey';
const MemoValue = 'MemoValue';
const WorkflowID = 'WorkflowID';

// A workflow that continues as new then terminates.
export async function workflow(input: string): Promise<string> {
  if (wf.workflowInfo().continuedFromExecutionRunId != undefined) {
    return input;
  }
  return await wf.continueAsNew<typeof workflow>(input);
}

export const feature = new Feature({
  workflow,
  execute: async (runner) => {
    return await runner.client.start(workflow, {
      taskQueue: runner.options.taskQueue,
      args: [InputData],
      memo: {
        MemoKey: MemoValue,
      },
      workflowId: WorkflowID,
      workflowExecutionTimeout: 60000,
    });
  },
  checkResult: async (runner, run) => {
    const result = await runner.waitForRunResult(run);
    assert.equal(result, InputData);
    // Workflow ID does not change after continue as new
    assert.equal(WorkflowID, run.workflowId);
    // Memos do not change after continue as new
    const executionDescription = await run.describe();
    const testMemo = executionDescription.memo?.[MemoKey] as string;
    assert.equal(testMemo, MemoValue);
  },
});
