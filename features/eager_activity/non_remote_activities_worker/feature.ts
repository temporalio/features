import * as wf from '@temporalio/workflow';
import { TimeoutType } from '@temporalio/common';
import { Feature } from '@temporalio/harness';

const activities = wf.proxyActivities<typeof activitiesImpl>({
  // Pick a long enough timeout for busy CI but not too long to get feedback quickly
  scheduleToCloseTimeout: '3 seconds',
});

export async function workflow(): Promise<void> {
  // Run a workflow that schedules a single activity with short schedule-to-close timeout
  try {
    await activities.dummy();
    throw new Error('Expected activity to time out');
  } catch (err) {
    // Catch activity failure in the workflow, check that it is caused by schedule-to-start timeout
    if (
      !(
        err instanceof wf.ActivityFailure &&
        err.cause instanceof wf.TimeoutFailure &&
        err.cause.timeoutType === TimeoutType.TIMEOUT_TYPE_SCHEDULE_TO_START
      )
    ) {
      throw err;
    }
  }
}

const activitiesImpl = {
  async dummy() {
    // noop
  },
};

// Start a worker with activities registered and non-local activities disabled
export const feature = new Feature({
  workerOptions: { enableNonLocalActivities: false },
  activities: activitiesImpl,
  workflow,
});
