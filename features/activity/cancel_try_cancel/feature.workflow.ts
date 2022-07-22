import type { feature } from './feature';
import { ActivityFailure, ApplicationFailure } from '@temporalio/common';
import {
  proxyActivities,
  sleep,
  CancellationScope,
  CancelledFailure,
  ActivityCancellationType,
  defineSignal,
  setHandler,
  condition,
} from '@temporalio/workflow';

// Allow 4 retries with no backoff
const { cancellableActivity } = proxyActivities<typeof feature.activities>({
  startToCloseTimeout: '1 minute',
  heartbeatTimeout: '5 seconds',
  // Disable retry
  retry: { maximumAttempts: 1 },
  cancellationType: ActivityCancellationType.TRY_CANCEL,
});

export const activityResultSignal = defineSignal<[string]>('activity-result');

export async function workflow(): Promise<void> {
  try {
    await CancellationScope.cancellable(async () => {
      // Start activity
      const actPromise = cancellableActivity();

      // Sleep for smallest amount of time (force task turnover)
      await sleep(1);

      // Cancel activity and await it
      CancellationScope.current().cancel();
      await actPromise;
    });
    throw new Error('Activity should have thrown cancellation error');
  } catch (e) {
    // Confirm the activity was cancelled
    if (!(e instanceof ActivityFailure) || !(e.cause instanceof CancelledFailure)) {
      const error = e instanceof Error ? e : new Error(`${e}`);
      throw new ApplicationFailure('Expected cancellable', null, true, undefined, error);
    }
  }

  // Confirm signal is received saying the activity got the cancel
  const activityResult = await new Promise<string>((resolve) => setHandler(activityResultSignal, resolve));
  if (activityResult != 'cancelled') {
    throw ApplicationFailure.nonRetryable(`Expected cancelled, got ${activityResult}`);
  }
}
