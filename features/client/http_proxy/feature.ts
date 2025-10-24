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
      connectionOpts: {
        ...runner.connectionOpts,
        ...(typeof runner.connectionOpts?.tls === 'object'
          ? {
              tls: {
                ...runner.connectionOpts.tls,
                clientCertPair: {
                  // Can't serialize Buffers safely to JSON, so let's cheat a bit
                  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
                  crt: Buffer.from(runner.connectionOpts.tls!.clientCertPair!.crt).toString('base64') as unknown as Buffer,
                  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
                  key: Buffer.from(runner.connectionOpts.tls!.clientCertPair!.key).toString('base64') as unknown as Buffer,
                },
              },
            }
          : undefined),
      },
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
        execArgv: ['-r', 'tsconfig-paths/register', __filename],
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
  const { connectionOpts, clientOpts, taskQueue } = JSON.parse(process.env.subprocess_opts) as SubprocessOpts;
  const connection = await Connection.connect({
    ...connectionOpts,
    ...(typeof connectionOpts?.tls === 'object'
      ? {
          tls: {
            ...connectionOpts.tls,
            // Can't serialize Buffers safely to JSON, so let's cheat a bit
            clientCertPair: {
              // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
              crt: Buffer.from(connectionOpts.tls!.clientCertPair!.crt as unknown as string, 'base64'),
              // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
              key: Buffer.from(connectionOpts.tls!.clientCertPair!.key as unknown as string, 'base64'),
            },
          },
        }
      : undefined),
  });
  try {
    const client = await new Client({
      ...clientOpts,
      connection,
    });
    await client.workflow.execute(workflow, {
      taskQueue,
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
