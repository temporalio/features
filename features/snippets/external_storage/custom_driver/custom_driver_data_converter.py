import dataclasses

from temporalio.converter import DataConverter, ExternalStorage

from custom_storage_driver import LocalDiskStorageDriver


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
