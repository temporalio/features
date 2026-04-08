import * as assert from 'assert';
import { randomUUID } from 'crypto';
import { Feature } from '@temporalio/harness';
import { ScheduleAlreadyRunning, ScheduleClient, Connection } from '@temporalio/client';

export async function workflow(): Promise<void> {}

export const feature = new Feature({
  workflow,
  alternateRun: async (runner) => {
    const connection = await Connection.connect({
      address: runner.options.address,
      tls: runner.options.tlsConfig,
    });

    const client = new ScheduleClient({
      connection,
      namespace: runner.options.namespace,
    });
    const createOpts = {
      scheduleId: `schedule-duplicate-error-${randomUUID()}`,
      spec: {
        intervals: [{ every: '1h' }],
      },
      action: {
        type: 'startWorkflow' as const,
        workflowType: workflow,
        taskQueue: runner.options.taskQueue,
      },
      state: {
        paused: true,
      },
    };
    const handle = await client.create(createOpts);

    try {
      // Creating again with the same schedule ID should throw ScheduleAlreadyRunning.
      await assert.rejects(
        () => client.create(createOpts),
        (err) => {
          assert.ok(err instanceof ScheduleAlreadyRunning, `expected ScheduleAlreadyRunning, got: ${err}`);
          return true;
        },
      );
    } finally {
      await handle.delete();
    }
  },
});
