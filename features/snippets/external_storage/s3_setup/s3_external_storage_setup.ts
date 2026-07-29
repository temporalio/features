/* eslint-disable @typescript-eslint/no-unused-vars */
// @@@SNIPSTART typescript-s3-external-storage-setup
import { Client, Connection } from '@temporalio/client';
import { ExternalStorage } from '@temporalio/common';
import { Worker } from '@temporalio/worker';

async function externalStorageSetup() {
  const dataConverter = {
    externalStorage: new ExternalStorage({ drivers: [driver] }),
  };

  const connection = await Connection.connect();
  const client = new Client({ connection, dataConverter });

  const worker = await Worker.create({
    workflowsPath: require.resolve('./workflows'),
    taskQueue: 'my-task-queue',
    dataConverter,
  });
}
// @@@SNIPEND

declare const driver: import('@temporalio/common').StorageDriver;
