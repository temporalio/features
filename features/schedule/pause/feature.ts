import { Feature } from '@temporalio/harness';
import { ScheduleClient, Connection } from '@temporalio/client';
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
    const scheduleId = `schedule-pause-${randomUUID()}`;
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
      const assertState = async (paused: boolean, note: string) => {
        const desc = await handle.describe();
        assert.equal(desc.state.paused, paused);
        assert.equal(desc.state.note, note);
      };

      // Confirm pause
      await assertState(true, '');
      // Re-pause
      await handle.pause('custom note1');
      await assertState(true, 'custom note1');
      // Unpause
      await handle.unpause();
      await assertState(false, 'Unpaused via TypeScript SDK"');
      // Re-unpause
      await handle.unpause('custom note2');
      await assertState(false, 'custom note2');
      // Pause
      await handle.pause();
      await assertState(true, 'Paused via TypeScript SDK"');

      return undefined;
    } finally {
      await handle.delete();
    }
  },
});
