import assert from 'assert';
import { fork } from 'child_process';
import { randomUUID } from 'crypto';
import { Client, ClientOptions, Connection, ConnectionOptions } from '@temporalio/client';
import { Feature } from '@temporalio/harness';

export async function workflow(): Promise<void> {
  return;
}

interface SubprocessOpts {
  connectionOpts: ConnectionOptions;
  clientOpts: ClientOptions;
  taskQueue: string;
}

export const feature = new Feature({
  workflow,

  async alternateRun(runner) {
    assert(runner.options.proxyUrl);

    const url = new URL(runner.options.proxyUrl);
    const proxyUrl = url.toString();

    // Proxying config is internally passed to @grpc/grpc-js using an environment variable.
    // We test in a subprocess to not infect other things in this process. The
    // subprocess will make the client call to run the workflow, this will just
    // return the run.
    const subprocessOpts: SubprocessOpts = {
      connectionOpts: runner.connectionOpts,
      clientOpts: runner.client.options,
      taskQueue: runner.options.taskQueue,
    };
    const childProcessPromise = new Promise((resolve, reject) => {
      const child = fork(__filename, {
        env: {
          ...process.env,
          grpc_proxy: proxyUrl,
          subprocess_opts: JSON.stringify(subprocessOpts),
        },
        execArgv: ['-r', 'ts-node/register', '-r', 'tsconfig-paths/register', __filename],
      });
      child.on('exit', (code) => {
        if (code !== 0) {
          reject(code);
        } else {
          resolve(undefined);
        }
      });
    });
    await Promise.race([runner.workerRunPromise, childProcessPromise]);
  },
});

async function subprocess() {
  if (typeof process.env.subprocess_opts !== 'string')
    throw new Error('Expected process.env.subprocess_opts to be a string');
  const subprocessOpts: SubprocessOpts = JSON.parse(process.env.subprocess_opts);
  const connection = await Connection.connect({
    ...subprocessOpts.connectionOpts,
  });
  try {
    const client = await new Client({
      ...subprocessOpts.clientOpts,
      connection,
    });
    await client.workflow.execute(workflow, {
      taskQueue: subprocessOpts.taskQueue,
      workflowId: `http-connect-proxy-${randomUUID()}`,
    });
  } finally {
    delete process.env.grpc_proxy;
    await connection.close();
  }
}

if (require.main === module) {
  subprocess().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
