import temporalio.converter
from temporalio.api.common.v1 import Payload
from temporalio.converter import ExternalStorage


def is_external_storage_enabled(workflow_id: str | None) -> bool:
    """Check whether external storage is enabled for this workflow via a feature flag service."""
    return True


def configure(my_driver):
    # @@@SNIPSTART python-external-storage-feature-flag-selector
    def feature_flag_selector(
        context: temporalio.converter.StorageDriverStoreContext, _payload: Payload
    ) -> temporalio.converter.StorageDriver | None:
        workflow_id = (
            context.target.id
            if isinstance(context.target, temporalio.converter.StorageDriverWorkflowInfo)
            else None
        )
        if is_external_storage_enabled(workflow_id):
            return my_driver
        return None

    ExternalStorage(
        drivers=[my_driver],
        driver_selector=feature_flag_selector,
    )
    # @@@SNIPEND
