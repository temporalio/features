import { Feature, retry } from '@temporalio/harness';
import { ScheduleClient, Connection, ScheduleOverlapPolicy } from '@temporalio/client';
import * as assert from 'assert';
import { randomUUID } from 'crypto';

export async function BasicWorkflow(arg: string): Promise<string> {
  return arg;
}

export const feature = new Feature({
  workflow: BasicWorkflow,
  alternateRun: async (runner) => {
    const connection = await Connection.connect({
      address: runner.options.address,
      tls: runner.options.tlsConfig,
    });

    const client = new ScheduleClient({
      connection,
      namespace: runner.options.namespace,
    });
    const scheduleId = `schedule-basic-${randomUUID()}`;
    const workflowId = `schedule-basic-workflow-${randomUUID()}`;
    const handle = await client.create({
      scheduleId,
      spec: {
        intervals: [{ every: '2s' }],
      },
      action: {
        type: 'startWorkflow',
        workflowId,
        workflowType: BasicWorkflow,
        args: ['arg1'],
        taskQueue: runner.options.taskQueue,
      },
      policies: {
        overlap: ScheduleOverlapPolicy.BUFFER_ONE,
      },
    });

    try {
      // Test describing
      const desc = await handle.describe();
      assert.equal(desc.scheduleId, scheduleId);
      assert.equal(desc.action.workflowId, workflowId);

      // Test listing
      // https://github.com/temporalio/sdk-typescript/issues/1013
      // Advanced visibility is eventually consistent. See https://github.com/temporalio/features/issues/182
      // assert.ok(
      //   await retry(async function () {
      //     for await (const schedule of client.list()) {
      //       if (schedule.scheduleId === scheduleId) {
      //         return true;
      //       }
      //     }
      //     return false;
      //   }, 10)
      // );

      const waitCompletedWith = async (untilResult: string) => {
        const iterable = await runner.client.list({
          query: "WorkflowType = 'BasicWorkflow'",
        });
        for await (const exec of iterable) {
          if (!exec.workflowId.startsWith(workflowId)) {
            continue;
          }
          if (exec.status.name == 'COMPLETED') {
            const wfHandle = runner.client.getHandle(exec.workflowId, exec.runId);
            const result = await wfHandle.result();
            if (result === untilResult) {
              return true;
            }
          } else {
            assert.equal(exec.status.name, 'RUNNING');
          }
        }
        return false;
      };

      // Wait for first completion
      assert.ok(await retry(() => waitCompletedWith('arg1'), 10));

      // Update
      await handle.update((x) => ({
        ...x,
        action: {
          type: 'startWorkflow',
          workflowId,
          workflowType: BasicWorkflow,
          args: ['arg2'],
          taskQueue: runner.options.taskQueue,
        },
      }));

      // Wait for second completion
      assert.ok(
        await retry(async function () {
          return waitCompletedWith('arg2');
        }, 10)
      );
    } finally {
      await handle.delete();
    }
  },
});
