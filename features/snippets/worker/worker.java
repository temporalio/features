import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.client.WorkflowClient;
import io.temporal.common.VersioningBehavior;
import io.temporal.common.WorkerDeploymentVersion;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerDeploymentOptions;
import io.temporal.worker.WorkerFactory;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.worker.WorkerOptions;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import io.temporal.workflow.WorkflowVersioningBehavior;
import java.util.concurrent.TimeUnit;

class WorkerSnippet {
  @ActivityInterface
  public interface GreetingActivities {
    @ActivityMethod
    String sayHello(String name);
  }

  public static class GreetingActivitiesImpl implements GreetingActivities {
    @Override
    public String sayHello(String name) {
      return "Hello, " + name + "!";
    }
  }

  @WorkflowInterface
  public interface GreetingWorkflow {
    @WorkflowMethod
    String greet(String name);
  }

  public static class GreetingWorkflowImpl implements GreetingWorkflow {
    @Override
    public String greet(String name) {
      return "Hello, " + name + "!";
    }
  }

  @WorkflowInterface
  public interface VersionedGreetingWorkflow {
    @WorkflowMethod
    String greet(String name);
  }

  // A versioning behavior is only valid on a Worker that has versioning enabled.
  public static class VersionedGreetingWorkflowImpl implements VersionedGreetingWorkflow {
    @Override
    @WorkflowVersioningBehavior(VersioningBehavior.PINNED)
    public String greet(String name) {
      return "Hello, " + name + "!";
    }
  }

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

  static void createWorker(WorkflowClient client) {
    // @@@SNIPSTART java-create-worker
    WorkerFactory factory = WorkerFactory.newInstance(client);

    Worker worker = factory.newWorker("my-task-queue");
    worker.registerWorkflowImplementationTypes(GreetingWorkflowImpl.class);
    worker.registerActivitiesImplementations(new GreetingActivitiesImpl());

    factory.start();
    // @@@SNIPEND
  }

  static void createVersionedWorker(WorkflowClient client) {
    WorkerFactory factory = WorkerFactory.newInstance(client);

    // @@@SNIPSTART java-versioned-worker
    WorkerOptions options =
        WorkerOptions.newBuilder()
            .setDeploymentOptions(
                WorkerDeploymentOptions.newBuilder()
                    .setVersion(new WorkerDeploymentVersion("my-app", "1.0"))
                    .setUseVersioning(true)
                    .build())
            .build();

    Worker worker = factory.newWorker("my-task-queue", options);
    worker.registerWorkflowImplementationTypes(VersionedGreetingWorkflowImpl.class);
    worker.registerActivitiesImplementations(new GreetingActivitiesImpl());
    // @@@SNIPEND

    factory.start();
  }

  static void shutdownWorker(WorkflowClient client) {
    WorkerFactory factory = WorkerFactory.newInstance(client);
    factory.newWorker("my-task-queue");
    factory.start();

    // @@@SNIPSTART java-worker-graceful-shutdown
    factory.shutdown();
    factory.awaitTermination(30, TimeUnit.SECONDS);
    // @@@SNIPEND
  }
}
