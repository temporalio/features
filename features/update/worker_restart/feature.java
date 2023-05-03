package update.worker_restart;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.*;
import org.junit.jupiter.api.Assertions;

import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Semaphore;

@ActivityInterface
public interface feature extends Feature {

  @WorkflowInterface
  interface IntWorkflow {
    @WorkflowMethod
    int workflow();

    @UpdateMethod
    int update(int i);

    @SignalMethod
    void finish();
  }

  @ActivityMethod
  void block();

  class Impl implements feature, IntWorkflow {

    private boolean doFinish = false;
    private int counter = 0;

    private static Semaphore updateStartedSemaphore = new Semaphore(0);
    private static Semaphore updateContinueSemaphore = new Semaphore(0);

    @Override
    public void block() {
      updateStartedSemaphore.release();
      updateContinueSemaphore.acquireUninterruptibly();
    }

    @Override
    public int workflow() {
      Workflow.await(() -> this.doFinish);
      return counter;
    }

    @Override
    public int update(int i) {
      var activities =
          activities(
              feature.class, builder -> builder.setScheduleToCloseTimeout(Duration.ofSeconds(10)));

      activities.block();
      var tmp = counter;
      counter += i;
      return tmp;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub =
          runner.client.newWorkflowStub(feature.IntWorkflow.class, run.execution.getWorkflowId());

      CompletableFuture<Integer> updateResult = CompletableFuture.supplyAsync(() -> stub.update(1));
      updateStartedSemaphore.acquireUninterruptibly();
      runner.getWorkerFactory();
      Thread.sleep(1000);
      runner.restartWorker();
      updateContinueSemaphore.release();

      Assertions.assertEquals(0, updateResult.get());
      stub.finish();
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(1, runner.waitForRunResult(run));
    }
  }
}
