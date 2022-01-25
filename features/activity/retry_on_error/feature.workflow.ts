import type { feature } from "./feature";
import { proxyActivities } from "@temporalio/workflow";

// Allow 4 retries with no backoff
const { alwaysFailActivity } = proxyActivities<typeof feature.activities>({
  startToCloseTimeout: "1 minute",
  retry: {
    // Retry immediately
    initialInterval: "1 millisecond",
    // Do not increase retry backoff each time
    backoffCoefficient: 1,
    // 5 total maximum attempts
    maximumAttempts: 5
  }
});

export async function workflow() {
  // Execute activity
  await alwaysFailActivity();
}
