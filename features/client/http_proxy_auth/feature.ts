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

    // Proxying config is internally passed to @grpc/grpc-js using an environment variable.
    // Here, we assert that the original value of that env var is properly restored afterward.
    process.env.grpc_proxy = 'foo';

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
      if (process.env.grpc_proxy !== 'foo') {
        throw new Error('Expected process.env.grpc_proxy to be foo');
      }
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
