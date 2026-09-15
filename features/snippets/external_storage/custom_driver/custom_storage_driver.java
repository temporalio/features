import io.temporal.api.common.v1.Payload;
import io.temporal.payload.storage.StorageDriver;
import io.temporal.payload.storage.StorageDriverClaim;
import io.temporal.payload.storage.StorageDriverRetrieveContext;
import io.temporal.payload.storage.StorageDriverStoreContext;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

// @@@SNIPSTART java-custom-storage-driver
class InMemoryStorageDriver implements StorageDriver {
  private static final String CLAIM_KEY = "key";

  private final ConcurrentMap<String, Payload> payloads = new ConcurrentHashMap<>();

  @Override
  public String getName() {
    return "in-memory-example";
  }

  @Override
  public String getType() {
    return "example.in-memory";
  }

  @Override
  public CompletableFuture<List<StorageDriverClaim>> store(
      StorageDriverStoreContext context, List<Payload> payloadsToStore) {
    context.getCancellationToken().throwIfCancellationRequested();

    List<StorageDriverClaim> claims = new ArrayList<>();
    for (Payload payload : payloadsToStore) {
      String key = UUID.randomUUID().toString();
      payloads.put(key, payload);
      claims.add(new StorageDriverClaim(Map.of(CLAIM_KEY, key)));
    }
    return CompletableFuture.completedFuture(claims);
  }

  @Override
  public CompletableFuture<List<Payload>> retrieve(
      StorageDriverRetrieveContext context, List<StorageDriverClaim> claims) {
    context.getCancellationToken().throwIfCancellationRequested();

    List<Payload> retrievedPayloads = new ArrayList<>();
    for (StorageDriverClaim claim : claims) {
      String key = claim.getClaimData().get(CLAIM_KEY);
      Payload payload = payloads.get(key);
      if (payload == null) {
        return failedFuture(new IllegalArgumentException("No payload for claim " + key));
      }
      retrievedPayloads.add(payload);
    }
    return CompletableFuture.completedFuture(retrievedPayloads);
  }

  private static <T> CompletableFuture<T> failedFuture(Throwable error) {
    CompletableFuture<T> result = new CompletableFuture<>();
    result.completeExceptionally(error);
    return result;
  }
}
// @@@SNIPEND
