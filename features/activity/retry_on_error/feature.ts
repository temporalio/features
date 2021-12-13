import { Context } from '@temporalio/activity';
import { WorkflowFailedError } from '@temporalio/client';
import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { workflow } from './feature.workflow';

export const feature = new Feature({
  workflow,
  activities: {
    alwaysFailActivity,
  },
  checkResult: async (runner, handle) => {
    await assert.rejects(runner.waitForRunResult(handle), (err) => {
      assert.ok(
        err instanceof WorkflowFailedError,
        `expected WorkflowFailedError, got ${typeof err}, message: ${(err as any).message}`
      );
      assert.equal(err.cause?.cause?.message, 'activity attempt 5 failed');
      return true;
    });
  },
});

async function alwaysFailActivity() {
  throw new Error(`activity attempt ${Context.current().info.attempt} failed`);
}
