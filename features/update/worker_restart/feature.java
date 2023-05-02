package update.worker_restart;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.*;
import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import org.junit.jupiter.api.Assertions;

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

    static final Object updateStartedLock = new Object();
    static Boolean updateStarted = false;

    static final Object updateContinueLock = new Object();
    static Boolean updateContinue = false;

    private static void signalUpdateStarted() {
      synchronized (updateStartedLock) {
        updateStarted = true;
        updateStartedLock.notify();
      }
    }

    private static void waitUpdateStarted() {
      synchronized (updateStartedLock) {
        while (!updateStarted) {
          try {
            updateStartedLock.wait();
          } catch (InterruptedException e) {
            throw new RuntimeException(e);
          }
        }
        updateStarted = false;
      }
    }

    private static void signalUpdateContinue() {
      synchronized (updateContinueLock) {
        updateContinue = true;
        updateContinueLock.notify();
      }
    }

    private static void waitUpdateContinue() {
      synchronized (updateContinueLock) {
        while (!updateContinue) {
          try {
            updateContinueLock.wait();
          } catch (InterruptedException e) {
            throw new RuntimeException(e);
          }
        }
        updateContinue = false;
      }
    }

    @Override
    public void block() {
      signalUpdateStarted();
      waitUpdateContinue();
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
      waitUpdateStarted();
      runner.getWorkerFactory().shutdown();
      Thread.sleep(1000);
      runner.restartWorker();
      signalUpdateContinue();

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
