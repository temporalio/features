import { Feature, retry } from '@temporalio/harness';
import { ScheduleClient, Connection } from '@temporalio/client';
import * as assert from 'assert';
import { randomUUID } from 'crypto';
import { setTimeout } from 'timers/promises'

export async function workflow(arg: string): Promise<string> {
  return arg;
}

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
    const scheduleId = `schedule-trigger-${randomUUID()}`;
    const handle = await client.create({
      scheduleId,
      spec: {
        intervals: [{ every: '1min' }],
      },
      action: {
        type: 'startWorkflow',
        workflowType: workflow,
        args: ['arg1'],
        taskQueue: runner.options.taskQueue,
      },
      state: {
        paused: true,
      },
    });

    try {
      await handle.trigger();
      // We have to wait before triggering again. See
      // https://github.com/temporalio/temporal/issues/3614
      await setTimeout(2000);
      await handle.trigger();
      await setTimeout(2000);

      assert.ok(
        await retry(async function () {
          return handle.describe().then((s) => {
            return s.info.numActionsTaken === 2;
          });
        })
      );
    } finally {
      await handle.delete();
    }
  },
});
