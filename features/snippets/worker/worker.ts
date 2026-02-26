import { NativeConnection, Worker } from '@temporalio/worker';

async function run() {
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
  
  await worker.run();
}