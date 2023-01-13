import { Feature, retry } from '@temporalio/harness';
import { ScheduleClient, Connection, ScheduleOverlapPolicy } from '@temporalio/client';
import * as assert from 'assert';
import { randomUUID } from 'crypto';

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
    const scheduleId = `schedule-backfill-${randomUUID()}`;
    const handle = await client.create({
      scheduleId,
      spec: {
        intervals: [{ every: '1m' }],
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
      const now = new Date();
      const threeYearsAgo = new Date(new Date(now).setFullYear(now.getFullYear() - 3));
      const thirtyMinutesAgo = new Date(new Date(now).setMinutes(now.getMinutes() - 30));

      await handle.backfill([
        {
          start: new Date(new Date(threeYearsAgo).setMinutes(threeYearsAgo.getMinutes() - 2)),
          end: threeYearsAgo,
          overlap: ScheduleOverlapPolicy.ALLOW_ALL,
        },
        {
          start: new Date(new Date(thirtyMinutesAgo).setMinutes(thirtyMinutesAgo.getMinutes() - 2)),
          end: thirtyMinutesAgo,
          overlap: ScheduleOverlapPolicy.ALLOW_ALL,
        },
      ]);

      assert.ok(
        await retry(async function () {
          return handle.describe().then((s) => {
            return s.info.numActionsTaken == 4 && s.info.runningActions.length == 0;
          });
        }, 10)
      );
    } finally {
      await handle.delete();
    }
  },
});
