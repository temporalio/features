import { Storage } from '@google-cloud/storage';
import { GcsStorageDriver } from '@temporalio/external-storage-gcs';
import { GoogleCloudGcsStorageDriverClient } from '@temporalio/external-storage-gcs-google-sdk';
/* eslint-disable @typescript-eslint/no-unused-vars */

{
  // @@@SNIPSTART typescript-gcs-driver-create
  const storage = new Storage();

  const driver = new GcsStorageDriver({
    client: new GoogleCloudGcsStorageDriverClient(storage),
    bucket: 'my-temporal-payloads',
  });
  // @@@SNIPEND
}
