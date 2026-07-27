import { Client, Connection } from '@temporalio/client';
import { ExternalStorage, type StorageDriver } from '@temporalio/common';
import { Worker } from '@temporalio/worker';
/* eslint-disable @typescript-eslint/no-unused-vars */

declare const driver: StorageDriver;

async function externalStorageSetup() {
  // @@@SNIPSTART typescript-s3-external-storage-setup
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
  // @@@SNIPEND
}
