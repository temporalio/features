import os

import aioboto3  # type: ignore[import-not-found]
from temporalio.contrib.aws.s3driver import S3StorageDriver
from temporalio.contrib.aws.s3driver.aioboto3 import new_aioboto3_client

AWS_PROFILE = os.environ.get("AWS_PROFILE")
AWS_REGION = os.environ.get("AWS_REGION", "us-east-2")


async def create_s3_driver():
    # @@@SNIPSTART python-s3-driver-create
    session = aioboto3.Session(profile_name=AWS_PROFILE, region_name=AWS_REGION)
    async with session.client("s3") as s3_client:
        driver = S3StorageDriver(
            client=new_aioboto3_client(s3_client),
            bucket="my-temporal-payloads",
        )
        # @@@SNIPEND
        return driver
