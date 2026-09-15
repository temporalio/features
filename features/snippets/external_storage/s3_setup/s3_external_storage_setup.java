import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.payload.storage.ExternalStorage;
import io.temporal.payload.storage.StorageDriver;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;

class S3ExternalStorageSetupSnippet {
  static void configureClientAndWorker(StorageDriver driver) {
    // @@@SNIPSTART java-s3-external-storage-setup
    ExternalStorage externalStorage = ExternalStorage.newBuilder().setDriver(driver).build();

    WorkflowServiceStubs service = WorkflowServiceStubs.newLocalServiceStubs();
    WorkflowClient client =
        WorkflowClient.newInstance(
            service,
            WorkflowClientOptions.newBuilder().setExternalStorage(externalStorage).build());
    WorkerFactory factory = WorkerFactory.newInstance(client);
    Worker worker = factory.newWorker("my-task-queue");
    // @@@SNIPEND
    factory.shutdown();
    service.shutdown();
  }
}
