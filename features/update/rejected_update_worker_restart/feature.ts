import * as assert from 'assert';
import { WorkflowUpdateFailedError } from '@temporalio/client';
import { Feature, waitForEvent } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

const add = wf.defineUpdate<number, [number]>('add');
const finish = wf.defineSignal('finish');

export async function workflow(): Promise<number> {
  let counter = 0;
  let finished = false;
  wf.setHandler(
    add,
    (value) => {
      counter += value;
      return counter;
    },
    {
      validator: (value) => {
        if (value < 0) throw new Error(`negative value: ${value}`);
      },
    },
  );
  wf.setHandler(finish, () => {
    finished = true;
  });
  await wf.condition(() => finished);
  return counter;
}

export const feature = new Feature({
  workflow,
  alternateRun: async (runner) => {
    const handle = await runner.executeSingleParameterlessWorkflow();
    await waitForEvent(
      () => runner.getHistoryEvents(handle),
      (event) => !!event.workflowTaskCompletedEventAttributes,
    );
    const before = await runner.getHistoryEvents(handle);

    await assert.rejects(handle.executeUpdate(add, { args: [-1] }), WorkflowUpdateFailedError);
    assert.deepStrictEqual(await runner.getHistoryEvents(handle), before, 'rejection changed history');

    runner.worker.shutdown();
    await runner.workerRunPromise;
    await runner.restartWorker();

    const completed = async () => {
      assert.equal(await handle.executeUpdate(add, { args: [1] }), 1);
      await handle.signal(finish);
      await runner.checkWorkflowResults(handle);
    };
    return await Promise.race([
      runner.workerRunPromise.then(() => {
        throw new Error('Worker exited before the feature completed');
      }),
      completed(),
    ]);
  },
  checkResult: async (_, handle) => {
    assert.equal(await handle.result(), 1);
  },
  checkHistory: async (runner, handle) => {
    const events = await runner.getHistoryEvents(handle);
    assert.equal(events.filter((event) => !!event.workflowExecutionUpdateAcceptedEventAttributes).length, 1);
    assert.equal(events.filter((event) => !!event.workflowExecutionUpdateCompletedEventAttributes).length, 1);
    assert.equal(events.filter((event) => !!event.workflowExecutionUpdateRejectedEventAttributes).length, 0);
  },
});
