import io.temporal.api.common.v1.Payload;
import io.temporal.payload.storage.StorageDriver;
import io.temporal.payload.storage.StorageDriverActivityInfo;
import io.temporal.payload.storage.StorageDriverClaim;
import io.temporal.payload.storage.StorageDriverRetrieveContext;
import io.temporal.payload.storage.StorageDriverStoreContext;
import io.temporal.payload.storage.StorageDriverTargetInfo;
import io.temporal.payload.storage.StorageDriverWorkflowInfo;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

// @@@SNIPSTART java-custom-storage-driver
class LocalDiskStorageDriver implements StorageDriver {
  private static final String CLAIM_PATH = "path";

  private final Path storeDir;

  LocalDiskStorageDriver(Path storeDir) {
    this.storeDir = storeDir;
  }

  @Override
  public String getName() {
    return "local-disk";
  }

  @Override
  public String getType() {
    return "local-disk";
  }

  @Override
  public CompletableFuture<List<StorageDriverClaim>> store(
      StorageDriverStoreContext context, List<Payload> payloads) {
    try {
      context.getCancellationToken().throwIfCancellationRequested();
      Path directory = storeDirectory(context);
      Files.createDirectories(directory);

      List<StorageDriverClaim> claims = new ArrayList<>();
      for (Payload payload : payloads) {
        context.getCancellationToken().throwIfCancellationRequested();
        Path file = directory.resolve(UUID.randomUUID() + ".bin");
        Files.write(file, payload.toByteArray());
        claims.add(new StorageDriverClaim(Map.of(CLAIM_PATH, file.toString())));
      }
      return CompletableFuture.completedFuture(claims);
    } catch (IOException e) {
      return failedFuture(new IllegalStateException("Could not write Payload", e));
    }
  }

  @Override
  public CompletableFuture<List<Payload>> retrieve(
      StorageDriverRetrieveContext context, List<StorageDriverClaim> claims) {
    try {
      context.getCancellationToken().throwIfCancellationRequested();

      List<Payload> payloads = new ArrayList<>();
      for (StorageDriverClaim claim : claims) {
        context.getCancellationToken().throwIfCancellationRequested();
        Path file = Paths.get(claim.getClaimData().get(CLAIM_PATH));
        payloads.add(Payload.parseFrom(Files.readAllBytes(file)));
      }
      return CompletableFuture.completedFuture(payloads);
    } catch (IOException e) {
      return failedFuture(new IllegalStateException("Could not read Payload", e));
    }
  }

  private Path storeDirectory(StorageDriverStoreContext context) {
    StorageDriverTargetInfo target = context.getTarget();
    if (target instanceof StorageDriverWorkflowInfo) {
      StorageDriverWorkflowInfo workflow = (StorageDriverWorkflowInfo) target;
      if (workflow.getId() != null) {
        return storeDir.resolve(workflow.getNamespace()).resolve(workflow.getId());
      }
    } else if (target instanceof StorageDriverActivityInfo) {
      StorageDriverActivityInfo activity = (StorageDriverActivityInfo) target;
      if (activity.getId() != null) {
        return storeDir.resolve(activity.getNamespace()).resolve(activity.getId());
      }
    }
    return storeDir;
  }

  private static <T> CompletableFuture<T> failedFuture(Throwable error) {
    CompletableFuture<T> result = new CompletableFuture<>();
    result.completeExceptionally(error);
    return result;
  }
}
// @@@SNIPEND
