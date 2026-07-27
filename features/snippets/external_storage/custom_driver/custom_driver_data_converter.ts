import { ExternalStorage } from '@temporalio/common';
import { LocalDiskStorageDriver } from './custom_storage_driver';
/* eslint-disable @typescript-eslint/no-unused-vars */

{
  // @@@SNIPSTART typescript-custom-driver-data-converter
  const dataConverter = {
    externalStorage: new ExternalStorage({
      drivers: [new LocalDiskStorageDriver()],
    }),
  };
  // @@@SNIPEND
}
