import dataclasses

from temporalio.converter import DataConverter, ExternalStorage


def configure(driver):
    # @@@SNIPSTART python-external-storage-threshold
    data_converter = dataclasses.replace(
        DataConverter.default,
        external_storage=ExternalStorage(
            drivers=[driver],
            payload_size_threshold=0,
        ),
    )
    # @@@SNIPEND
    return data_converter
