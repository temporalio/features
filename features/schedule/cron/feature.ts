import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';
import { temporal } from '@temporalio/proto';

export async function workflow(): Promise<void> {
  console.log('CRON SCHED', wf.workflowInfo().cronSchedule);
  if (wf.workflowInfo().cronSchedule != '@every 2s') {
    throw new Error('Invalid cron schedule');
  }
}

export const feature = new Feature({
  workflow,
  workflowStartOptions: { cronSchedule: '@every 2s' },
  checkResult: async (runner, handle) => {
    try {
      // Try 10 times (waiting 1s before each) to get at least 2 completions
      for (let i = 0; i < 10; i++) {
        await new Promise((r) => setTimeout(r, 1000));
        const resp = await runner.client.workflowService.listWorkflowExecutions({
          namespace: runner.options.namespace,
          query: `WorkflowId = '${handle.workflowId}'`,
        });
        const completed = resp.executions.filter((info) => {
          if (info.status == temporal.api.enums.v1.WorkflowExecutionStatus.WORKFLOW_EXECUTION_STATUS_COMPLETED) {
            return true;
          } else if (info.status != temporal.api.enums.v1.WorkflowExecutionStatus.WORKFLOW_EXECUTION_STATUS_RUNNING) {
            throw new Error('Not running');
          }
          return false;
        });
        if (completed.length >= 2) {
          return;
        }
      }
      throw new Error('Did not get at least 2 completed');
    } finally {
      // Terminate on complete
      try {
        await handle.terminate('feature complete');
      } catch (err) {
        console.warn('Failed terminating workflow', err);
      }
    }
  },
});
