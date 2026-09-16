import io.temporal.payload.storage.ExternalStorage;
import io.temporal.payload.storage.StorageDriver;

class ThresholdConfigSnippet {
  static ExternalStorage configure(StorageDriver driver) {
    // @@@SNIPSTART java-external-storage-threshold
    return ExternalStorage.newBuilder().setDriver(driver).setPayloadSizeThreshold(0).build();
    // @@@SNIPEND
  }
}
