import { ExternalStorage, type StorageDriver } from '@temporalio/common';
/* eslint-disable @typescript-eslint/no-unused-vars */

declare const driver: StorageDriver;

{
  // @@@SNIPSTART typescript-external-storage-threshold
  const dataConverter = {
    externalStorage: new ExternalStorage({
      drivers: [driver],
      payloadSizeThreshold: 0,
    }),
  };
  // @@@SNIPEND
}
