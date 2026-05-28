import * as assert from 'assert';
import { Feature, waitForEvent } from '@temporalio/harness';
import { proxyActivities, sleep } from '@temporalio/workflow';

const WORKFLOW_COUNT = 5;
const SHUTDOWN_TIMEOUT_MS = 5000;

const activities = {
  async noop(): Promise<void> {},
};

const activity = proxyActivities<typeof activities>({
  scheduleToCloseTimeout: '10s',
  startToCloseTimeout: '5s',
  retry: { maximumAttempts: 1 },
});

export async function workflow(): Promise<void> {
  for (;;) {
    await sleep(20);
    await activity.noop();
  }
}

export const feature = new Feature({
  workflow,
  activities,
  workerOptions: { shutdownGraceTime: '10s' },
  alternateRun: async (runner) => {
    const handles = [];
    for (let i = 0; i < WORKFLOW_COUNT; i++) {
      handles.push(await runner.executeParameterlessWorkflow('workflow'));
    }

    try {
      for (const handle of handles) {
        await waitForEvent(
          () => runner.getHistoryEvents(handle),
          (event) => !!event.activityTaskScheduledEventAttributes,
          10000,
          100,
        );
      }

      const start = Date.now();
      runner.worker.shutdown();
      await runner.workerRunPromise;
      assert.ok(Date.now() - start <= SHUTDOWN_TIMEOUT_MS, 'worker shutdown exceeded timeout');

      if (expectWorkerPollCompleteOnShutdown()) {
        for (const handle of handles) {
          const events = await runner.getHistoryEvents(handle);
          const problem = events.find(
            (event) => event.workflowTaskFailedEventAttributes || event.workflowTaskTimedOutEventAttributes,
          );
          assert.equal(problem, undefined, 'workflow task failed or timed out');
        }
      }
    } finally {
      for (const handle of handles) {
        try {
          await handle.terminate('feature cleanup');
        } catch {
          // Ignore cleanup races.
        }
      }
    }
  },
});

function expectWorkerPollCompleteOnShutdown(): boolean {
  const capabilitiesJson = process.env.FEATURE_NAMESPACE_CAPABILITIES;
  assert.ok(capabilitiesJson, 'FEATURE_NAMESPACE_CAPABILITIES is required');
  const capabilities = JSON.parse(capabilitiesJson) as Record<string, boolean>;
  assert.ok(
    Object.prototype.hasOwnProperty.call(capabilities, 'workerPollCompleteOnShutdown'),
    'FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown',
  );
  return capabilities.workerPollCompleteOnShutdown;
}
