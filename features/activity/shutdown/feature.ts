import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import { setTimeout } from 'timers/promises';
import { ActivityFailure, ApplicationFailure, TimeoutFailure, TimeoutType } from '@temporalio/common';
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

const activities = wf.proxyActivities<typeof activitiesImpl>({
  scheduleToCloseTimeout: '300 ms',
  retry: { maximumAttempts: 1 },
});

export async function workflow(): Promise<string> {
  const fut = activities.cancelSuccess();
  const fut1 = activities.cancelFailure();
  const fut2 = activities.cancelIgnore();

  await fut;

  await assert.rejects(fut1, (err: unknown) => {
    assert.ok(
      err instanceof wf.ActivityFailure &&
        err.cause instanceof ApplicationFailure &&
        err.cause.message?.includes('worker is shutting down')
    );
    return true;
  });

  await assert.rejects(fut2, (err: unknown) => {
    assert.ok(
      err instanceof wf.ActivityFailure &&
        err.cause instanceof TimeoutFailure &&
        err.cause.timeoutType === TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_CLOSE
    );
    return true;
  });

  return 'done';
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
    await setTimeout(15000);
  },
};

export const feature = new Feature({
  workflow,
  activities: activitiesImpl,
  workerOptions: { shutdownGraceTime: '1s' },
  async execute(runner) {
    const handle = await runner.executeSingleParameterlessWorkflow();
    await setTimeout(100);
    notifyShutdown();
    runner.worker.shutdown();
    await runner.workerRunPromise;
    await runner.restartWorker();
    return handle;
  },
  async checkResult(runner, handle) {
    assert.equal(await handle.result(), 'done');
  },
});