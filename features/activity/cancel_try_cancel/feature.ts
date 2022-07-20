import { Context } from '@temporalio/activity';
import { WorkflowClient } from '@temporalio/client';
import { CancelledFailure } from '@temporalio/common';
import { Feature } from '@temporalio/harness';
import { workflow, activityResultSignal } from './feature.workflow';

export const feature = new Feature({
  workflow,
  activities: {
    cancellableActivity,
  },
  execute: async (runner) => {
    client = runner.client;
    return await runner.executeSingleParameterlessWorkflow();
  },
});

let client: WorkflowClient | undefined;

async function cancellableActivity() {
  // Expect client to be set
  if (!client) {
    throw new Error('Missing client');
  }

  // Heartbeat every second for a minute
  let result = 'timeout';
  for (let i = 0; i < 60; i++) {
    // Wait for a second or until cancelled
    try {
      await Promise.race([Context.current().sleep(1000), Context.current().cancelled]);
    } catch (e) {
      // Exit loop if cancelled or rethrow if other error
      if (e instanceof CancelledFailure) {
        result = 'cancelled';
        break;
      }
      throw e;
    }
    await Context.current().sleep(1000);

    // Heartbeat
    Context.current().heartbeat();
  }

  // Send to signal
  await client.getHandle(Context.current().info.workflowExecution.workflowId).signal(activityResultSignal, result);
}
