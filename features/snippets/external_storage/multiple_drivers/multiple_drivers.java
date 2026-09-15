import io.temporal.payload.storage.ExternalStorage;
import io.temporal.payload.storage.StorageDriver;
import java.util.Arrays;

class MultipleDriversSnippet {
  static ExternalStorage configure(StorageDriver preferredDriver, StorageDriver legacyDriver) {
    // @@@SNIPSTART java-external-storage-multiple-drivers
    return ExternalStorage.newBuilder()
        .setDrivers(Arrays.asList(preferredDriver, legacyDriver))
        .setDriverSelector((context, payload) -> preferredDriver)
        .build();
    // @@@SNIPEND
  }
}
