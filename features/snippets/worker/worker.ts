import { NativeConnection, Worker } from '@temporalio/worker';
/* eslint-disable @typescript-eslint/no-unused-vars */

async function _run() {
  const connection = await NativeConnection.connect({
    address: 'localhost:7233',
  });

  // @@@SNIPSTART typescript-worker-max-cached-workflows
  const worker = await Worker.create({
    connection,
    taskQueue: 'task-queue',
    maxCachedWorkflows: 0,
  });
  // @@@SNIPEND
}

async function _runVersioned() {
  const connection = await NativeConnection.connect({
    address: 'localhost:7233',
  });

  // @@@SNIPSTART typescript-versioned-worker
  const worker = await Worker.create({
    connection,
    taskQueue: 'my-task-queue',
    workflowsPath: require.resolve('./workflows'),
    workerDeploymentOptions: {
      version: { deploymentName: 'my-app', buildId: '1.0' },
      useWorkerVersioning: true,
      defaultVersioningBehavior: 'PINNED',
    },
  });
  // @@@SNIPEND
}

// A Serverless Worker on GCP Cloud Run is a standard long-lived Worker that reads its
// connection settings from the environment and enables Worker Versioning.
async function _runCloudRunWorker() {
  // @@@SNIPSTART typescript-cloud-run-worker
  const connection = await NativeConnection.connect({
    address: process.env.TEMPORAL_ADDRESS,
    apiKey: process.env.TEMPORAL_API_KEY,
    tls: true,
  });

  const worker = await Worker.create({
    connection,
    namespace: process.env.TEMPORAL_NAMESPACE!,
    taskQueue: process.env.TEMPORAL_TASK_QUEUE!,
    workflowsPath: require.resolve('./workflows'),
    workerDeploymentOptions: {
      version: { deploymentName: 'my-app', buildId: 'build-1' },
      useWorkerVersioning: true,
      defaultVersioningBehavior: 'PINNED',
    },
  });

  await worker.run();
  // @@@SNIPEND
}

async function _runWithGracefulShutdown() {
  const connection = await NativeConnection.connect({
    address: 'localhost:7233',
  });

  // @@@SNIPSTART typescript-worker-graceful-shutdown
  const worker = await Worker.create({
    connection,
    taskQueue: 'my-task-queue',
    workflowsPath: require.resolve('./workflows'),
    shutdownGraceTime: '30s',
  });

  await worker.run();
  // @@@SNIPEND
}
