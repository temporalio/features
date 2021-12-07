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
    try {
      await runner.waitForRunResult(handle);
      throw new Error('expected failure');
    } catch (err) {
      assert.ok(err instanceof WorkflowFailedError);
      assert.equal(err.cause?.cause?.message, 'activity attempt 5 failed');
    }
  },
});

async function alwaysFailActivity() {
  throw new Error(`activity attempt ${Context.current().info.attempt} failed`);
}
