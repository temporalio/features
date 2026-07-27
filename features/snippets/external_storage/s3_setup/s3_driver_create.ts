import { S3Client } from '@aws-sdk/client-s3';
import { S3StorageDriver } from '@temporalio/external-storage-s3';
import { AwsSdkS3StorageDriverClient } from '@temporalio/external-storage-s3-aws-sdk';
/* eslint-disable @typescript-eslint/no-unused-vars */

{
  // @@@SNIPSTART typescript-s3-driver-create
  const s3Client = new S3Client({ region: 'us-east-2' });

  const driver = new S3StorageDriver({
    client: new AwsSdkS3StorageDriverClient(s3Client),
    bucket: 'my-temporal-payloads',
  });
  // @@@SNIPEND
}
