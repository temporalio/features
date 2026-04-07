from temporalio.converter import ExternalStorage

from temporalio.contrib.aws.s3driver import S3StorageDriver


def configure(s3_client, legacy_driver):
    # @@@SNIPSTART python-external-storage-multiple-drivers
    preferred_driver = S3StorageDriver(client=s3_client, bucket="my-bucket")
    legacy_driver = LegacyStorageDriver()

    ExternalStorage(
        drivers=[preferred_driver, legacy_driver],
        driver_selector=lambda context, payload: preferred_driver,
    )
    # @@@SNIPEND
