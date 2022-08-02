import { Context } from '@temporalio/activity';
import { WorkflowClient } from '@temporalio/client';
import { CancelledFailure, ActivityFailure, ApplicationFailure } from '@temporalio/common';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

const { cancellableActivity } = wf.proxyActivities<typeof activitiesImpl>({
  startToCloseTimeout: '1 minute',
  heartbeatTimeout: '5 seconds',
  // Disable retry
  retry: { maximumAttempts: 1 },
  cancellationType: wf.ActivityCancellationType.TRY_CANCEL,
});

export const activityResultSignal = wf.defineSignal<[string]>('activity-result');

export async function workflow(): Promise<void> {
  try {
    await wf.CancellationScope.cancellable(async () => {
      // Start activity
      const actPromise = cancellableActivity();

      // Sleep for smallest amount of time (force task turnover)
      await wf.sleep(1);

      // Cancel activity and await it
      wf.CancellationScope.current().cancel();
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
  const activityResult = await new Promise<string>((resolve) => wf.setHandler(activityResultSignal, resolve));
  if (activityResult != 'cancelled') {
    throw ApplicationFailure.nonRetryable(`Expected cancelled, got ${activityResult}`);
  }
}

let client: WorkflowClient | undefined;

const activitiesImpl = {
  async cancellableActivity() {
    // Expect client to be set
    if (!client) {
      throw new Error('Missing client');
    }

    // Heartbeat every second for a minute
    let result = 'timeout';
    for (let i = 0; i < 60; i++) {
      // Wait for a second or until cancelled
      try {
        await Context.current().sleep(1000);
      } catch (e) {
        // Exit loop if cancelled or rethrow if other error
        if (e instanceof CancelledFailure) {
          result = 'cancelled';
          break;
        }
        throw e;
      }

      // Heartbeat
      Context.current().heartbeat();
    }

    // Send to signal
    await client.getHandle(Context.current().info.workflowExecution.workflowId).signal(activityResultSignal, result);
  },
};

export const feature =
  !wf.inWorkflowContext() &&
  new Feature({
    workflow,
    activities: activitiesImpl,
    execute: async (runner) => {
      client = runner.client;
      return await runner.executeSingleParameterlessWorkflow();
    },
  });
