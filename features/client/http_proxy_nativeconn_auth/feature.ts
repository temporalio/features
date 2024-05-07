import assert from 'assert';
import { randomUUID } from 'crypto';
import { Feature } from '@temporalio/harness';
import { NativeConnection, Worker } from '@temporalio/worker';

export async function workflow(): Promise<void> {
  return;
}

export const feature = new Feature({
  workflow,
  async alternateRun(runner) {
    assert(runner.options.proxyUrl);

    // Use a different task queue, to avoid the workflow being picked up by the runner's non-proxied worker
    const taskQueue = `${runner.options.taskQueue}-2`;
    const connection = await NativeConnection.connect({
      ...runner.nativeConnectionOpts,
      proxy: {
        type: 'http-connect',
        targetHost: runner.options.proxyUrl,
        basicAuth: {
          username: 'proxy-user',
          password: 'proxy-pass',
        },
      },
    });
    try {
      const worker = await Worker.create({
        ...runner.workerOpts,
        taskQueue,
        connection,
      });
      await worker.runUntil(async () => {
        await runner.client.workflow.execute(workflow, {
          taskQueue,
          workflowId: `http-connect-proxy-${randomUUID()}`,
        });
      });
    } finally {
      await connection.close();
    }
  },
});
