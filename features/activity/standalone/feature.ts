import * as assert from 'assert';
import { randomUUID } from 'crypto';
import { Feature } from '@temporalio/harness';

export async function workflow(): Promise<void> {}

const activities = {
  async echo(value: string): Promise<string> {
    return value;
  },
};

export const feature = new Feature({
  workflow,
  activities,
  alternateRun: async (runner) => {
    const handle = await runner.client.activity.start('echo', {
      id: randomUUID(),
      taskQueue: runner.options.taskQueue,
      startToCloseTimeout: '1 minute',
      args: ['hello'],
    });

    assert.equal(await handle.result(), 'hello');

    const desc = await handle.describe();
    assert.equal(desc.status, 'COMPLETED');
  },
});
