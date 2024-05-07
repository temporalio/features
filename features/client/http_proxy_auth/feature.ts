import assert from 'assert';
import { randomUUID } from 'crypto';
import { Client, Connection } from '@temporalio/client';
import { Feature } from '@temporalio/harness';

export async function workflow(): Promise<void> {
  return;
}

export const feature = new Feature({
  workflow,
  async alternateRun(runner) {
    assert(runner.options.proxyUrl);

    const connection = await Connection.connect({
      ...runner.connectionOpts,
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
      const client = await new Client({
        ...runner.client.options,
        connection,
      });
      await Promise.race([
        client.workflow.execute(workflow, {
          taskQueue: `${runner.options.taskQueue}`,
          workflowId: `http-connect-proxy-${randomUUID()}`,
        }),
        runner.workerRunPromise,
      ]);
    } finally {
      delete process.env.grpc_proxy;
      await connection.close();
    }
  },
});
