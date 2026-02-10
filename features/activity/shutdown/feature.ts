import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import { ApplicationFailure, TimeoutFailure, TimeoutType } from '@temporalio/common';
import * as assert from 'assert';

// Promise and helper used for activities to detect worker shutdown
let shutdownRequested = false;
let notifyShutdown: () => void;
const shutdownPromise = new Promise<void>((resolve) => {
  notifyShutdown = () => {
    shutdownRequested = true;
    resolve();
  };
});

function waitForShutdown(): Promise<void> {
  return shutdownRequested ? Promise.resolve() : shutdownPromise;
}

const activitiesImpl = {
  async cancelSuccess(): Promise<void> {
    await waitForShutdown();
  },
  async cancelFailure(): Promise<void> {
    await waitForShutdown();
    throw new Error('worker is shutting down');
  },
  async cancelIgnore(): Promise<void> {
    // Use a plain setTimeout that doesn't respond to activity cancellation,
    // so the worker must abandon this activity on shutdown.
    await new Promise((resolve) => setTimeout(resolve, 15000));
  },
};

const gracefulActivities = wf.proxyActivities<typeof activitiesImpl>({
  scheduleToCloseTimeout: '30s',
  retry: { maximumAttempts: 1 },
});

const ignoringActivities = wf.proxyActivities<typeof activitiesImpl>({
  scheduleToCloseTimeout: '300ms',
  retry: { maximumAttempts: 1 },
});

export async function workflow(): Promise<string> {
  const fut = gracefulActivities.cancelSuccess();
  const fut1 = gracefulActivities.cancelFailure();
  const fut2 = ignoringActivities.cancelIgnore();

  await fut;

  try {
    await fut1;
  } catch (e) {
    if (
      !(e instanceof wf.ActivityFailure) ||
      !(e.cause instanceof ApplicationFailure) ||
      !e.cause.message?.includes('worker is shutting down')
    ) {
      const error = e instanceof Error ? e : new Error(`${e}`);
      throw new ApplicationFailure('Unexpected error for cancelFailure', null, true, undefined, error);
    }
  }

  try {
    await fut2;
  } catch (e) {
    if (
      !(e instanceof wf.ActivityFailure) ||
      !(e.cause instanceof TimeoutFailure) ||
      e.cause.timeoutType !== TimeoutType.SCHEDULE_TO_CLOSE
    ) {
      const error = e instanceof Error ? e : new Error(`${e}`);
      throw new ApplicationFailure('Unexpected error for cancelIgnore', null, true, undefined, error);
    }
  }

  return 'done';
}

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  workerOptions: { shutdownGraceTime: '1s' },
  alternateRun: async (runner) => {
    const handle = await runner.executeSingleParameterlessWorkflow();

    // Wait for activity task to be scheduled
    await (
      await import('@temporalio/harness')
    ).waitForEvent(
      () => runner.getHistoryEvents(handle),
      (event) => !!event.activityTaskScheduledEventAttributes,
      5000, // 5 second timeout
      100 // 100ms poll interval
    );

    notifyShutdown();

    runner.worker.shutdown();
    await runner.workerRunPromise;

    await runner.restartWorker();
    return await Promise.race([runner.workerRunPromise, runner.checkWorkflowResults(handle)]);
  },
  async checkResult(runner, handle) {
    assert.equal(await handle.result(), 'done');
  },
});
