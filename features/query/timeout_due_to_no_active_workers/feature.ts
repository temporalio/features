import { status } from '@grpc/grpc-js';
import * as wf from '@temporalio/workflow';
import { Feature } from '@temporalio/harness';
import { ServiceError } from '@temporalio/client';
import * as assert from 'assert';

const finishSignal = wf.defineSignal('finish');
const query = wf.defineQuery<boolean>('somequery');

export async function workflow(): Promise<void> {
  wf.setHandler(query, () => {
    return true;
  });

  await new Promise((resolve) => wf.setHandler(finishSignal, () => resolve(null)));
}

export const feature = new Feature({
  workflow,
  alternateRun: async (runner) => {
    // Start the workflow
    const wfHandle = await runner.executeSingleParameterlessWorkflow();
    // Query to make sure the workflow has processed one task
    await wfHandle.query(query);
    // Shutdown the worker
    runner.worker.shutdown();
    await runner.workerRunPromise;
    // Make a query, it will time out
    try {
      await runner.client.withDeadline(new Date(Date.now() + 1000), () => wfHandle.query(query));
    } catch (e) {
      assert.ok(e instanceof ServiceError);
      const reAnyd = e as any;
      // Can be cancelled or deadline exceeded depending on whether client or
      // server hit timeout first in a racy way
      assert.ok(reAnyd.cause?.code === status.DEADLINE_EXCEEDED || reAnyd.cause?.code === status.CANCELLED);
    }
    // Restart worker to finish the workflow
    await runner.restartWorker();
    await wfHandle.signal(finishSignal);
    return await Promise.race([runner.workerRunPromise, runner.checkWorkflowResults(wfHandle)]);
  },
});
