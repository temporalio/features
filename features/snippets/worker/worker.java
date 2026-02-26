import io.temporal.client.WorkflowClient;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import io.temporal.worker.WorkerFactoryOptions;

class WorkerSnippet {
  public static void main(String[] args) {
    WorkflowServiceStubs service = WorkflowServiceStubs.newLocalServiceStubs();
    WorkflowClient client = WorkflowClient.newInstance(service);

    // @@@SNIPSTART java-worker-max-cached-workflows
    WorkerFactory factory =
        WorkerFactory.newInstance(
            client, WorkerFactoryOptions.newBuilder().setWorkflowCacheSize(0).build());
    Worker worker = factory.newWorker("task-queue");
    // @@@SNIPEND

    factory.start();
  }
}
