import type { feature } from './feature';
import { proxyActivities } from '@temporalio/workflow';

export async function workflow() {
  // Allow 4 retries with no backoff
  const activities = proxyActivities<typeof feature.activities>({
    startToCloseTimeout: '1 minute',
    retry: {
      // Retry immediately
      initialInterval: '1 millisecond',
      // Do not increase retry backoff each time
      backoffCoefficient: 1,
      // 5 total maximum attempts
      maximumAttempts: 5,
    },
  });

  // Execute activity
  await activities.alwaysFailActivity();
}
