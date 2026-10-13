import type { S3Client } from '@aws-sdk/client-s3';
import { ExternalStorage, type StorageDriver } from '@temporalio/common';
import { S3StorageDriver } from '@temporalio/external-storage-s3';
import { AwsSdkS3StorageDriverClient } from '@temporalio/external-storage-s3-aws-sdk';
/* eslint-disable @typescript-eslint/no-unused-vars */

declare const s3Client: S3Client;
declare const LegacyStorageDriver: new () => StorageDriver;

{
  // @@@SNIPSTART typescript-external-storage-multiple-drivers
  const preferredDriver = new S3StorageDriver({
    client: new AwsSdkS3StorageDriverClient(s3Client),
    bucket: 'my-bucket',
  });
  const legacyDriver = new LegacyStorageDriver();

  const externalStorage = new ExternalStorage({
    drivers: [preferredDriver, legacyDriver],
    driverSelector: () => preferredDriver,
  });
  // @@@SNIPEND
}
