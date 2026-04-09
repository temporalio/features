import dataclasses

from custom_storage_driver import (  # type: ignore[import-not-found]
    LocalDiskStorageDriver,
)
from temporalio.converter import DataConverter, ExternalStorage


def configure():
    # @@@SNIPSTART python-custom-driver-data-converter
    data_converter = dataclasses.replace(
        DataConverter.default,
        external_storage=ExternalStorage(
            drivers=[LocalDiskStorageDriver()],
        ),
    )
    # @@@SNIPEND
    return data_converter
