import { Context } from '@temporalio/activity';
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
    await Context.current().sleep(15000);
  },
};

const activities = wf.proxyActivities<typeof activitiesImpl>({
  scheduleToCloseTimeout: '300ms',
  retry: { maximumAttempts: 1 },
});

export async function workflow(): Promise<string> {
  const fut = activities.cancelSuccess();
  const fut1 = activities.cancelFailure();
  const fut2 = activities.cancelIgnore();

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
