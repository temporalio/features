package update.worker_restart;

import io.temporal.activity.ActivityInterface;
import io.temporal.activity.ActivityMethod;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.Workflow;
import org.junit.jupiter.api.Assertions;
import update.updateutil.UpdateUtil;

import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

@ActivityInterface
public interface feature extends Feature, IntWorkflow {

  @ActivityMethod
  void block();

  @UpdateMethod()
  int update(int i);

  @SignalMethod
  void finish();

  class Impl implements feature {

    private boolean doFinish = false;
    private int counter = 0;

    static Object updateStartedLock = new Object();
    static Boolean updateStarted = false;

    static Object updateContinueLock = new Object();
    static Boolean updateContinue = false;

    private static void signalUpdateStarted() {
      synchronized (updateStartedLock) {
        updateStarted = true;
        updateStartedLock.notify();
      }
    }

    private static void waitUpdateStarted() {
      synchronized (updateStartedLock) {
        while(!updateStarted) {
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
        while(!updateContinue) {
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
      var activities = activities(feature.class, builder -> builder
        .setScheduleToCloseTimeout(Duration.ofSeconds(10)));

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
    public Run execute(Runner runner) {
      String reason = UpdateUtil.CheckServerSupportsUpdate(runner.client);
      if (!reason.isEmpty()) {
        runner.Skip(reason);
      }

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      CompletableFuture<Integer> updateResult = CompletableFuture.supplyAsync(() -> stub.update(1));
      waitUpdateStarted();
      runner.getWorkerFactory().shutdown();
      try {
        Thread.sleep(1000);
      } catch (InterruptedException e) {
        throw new RuntimeException(e);
      }
      runner.restartWorker();
      signalUpdateContinue();

      try {
        Assertions.assertEquals(0, updateResult.get());
      } catch (InterruptedException e) {
        throw new RuntimeException(e);
      } catch (ExecutionException e) {
        throw new RuntimeException(e);
      }
      stub.finish();
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(1, runner.waitForRunResult(run));
    }
  }
}
