import { Context } from '@temporalio/activity';
import * as wf from '@temporalio/workflow';
import { WorkflowFailedError } from '@temporalio/client';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';

// Allow 4 retries with no backoff
const activities = wf.proxyActivities<typeof activitiesImpl>({
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

export async function workflow(): Promise<void> {
  // Execute activity
  await activities.alwaysFailActivity();
}

const activitiesImpl = {
  async alwaysFailActivity() {
    throw new Error(`activity attempt ${Context.current().info.attempt} failed`);
  },
};

export const feature =
  wf.inWorkflowContext() ||
  new Feature({
    workflow,
    activities: activitiesImpl,
    checkResult: async (runner, handle) => {
      await assert.rejects(runner.waitForRunResult(handle), (err) => {
        assert.ok(
          err instanceof WorkflowFailedError,
          `expected WorkflowFailedError, got ${typeof err}, message: ${(err as any).message}`
        );
        assert.equal(err.cause?.cause?.message, 'activity attempt 5 failed');
        return true;
      });
    },
  });
