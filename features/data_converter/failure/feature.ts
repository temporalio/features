import { Feature } from '@temporalio/harness';
import * as assert from 'assert';
import { ApplicationFailure } from '@temporalio/common';

// Run a workflow that fails
export async function workflow(): Promise<void> {
  const e = Error('cause error');
  e.stack = 'cause stack trace';
  const applicationError = new ApplicationFailure('main error', null, true, undefined, e);
  applicationError.stack = 'main stack trace';
  throw applicationError;
}

export const feature = new Feature({
  workflow,
  async checkResult(runner, handle) {
    try {
      await handle.result();
      // Workflow should fail
      assert.fail();
    } catch (error) {
      // get result payload of an WorkflowExecutionFailed event from workflow history
      const events = await runner.getHistoryEvents(handle);
      const completedEvent = events.find(
        ({ workflowExecutionFailedEventAttributes }) => !!workflowExecutionFailedEventAttributes
      );

      const failure = completedEvent?.workflowExecutionFailedEventAttributes?.failure;
      assert.ok(failure);
      assert.equal('Encoded failure', failure.message);
      assert.equal('', failure.stackTrace);
      assert.equal('json/plain', failure.encodedAttributes?.metadata?.['encoding']);
      assert.equal('main error', JSON.parse(failure.encodedAttributes?.data?.toString() ?? '')['message']);
      assert.equal('main stack trace', JSON.parse(failure.encodedAttributes?.data?.toString() ?? '')['stack_trace']);
      const cause = failure.cause;
      assert.ok(cause);
      assert.equal('Encoded failure', cause.message);
      assert.equal('', cause.stackTrace);
      assert.equal('json/plain', cause.encodedAttributes?.metadata?.['encoding']);
      assert.equal('cause error', JSON.parse(cause.encodedAttributes?.data?.toString() ?? '')['message']);
      assert.equal('cause stack trace', JSON.parse(cause.encodedAttributes?.data?.toString() ?? '')['stack_trace']);
    }
  },
  dataConverter: {
    failureConverterPath: __dirname + '/failure_converter.js',
  },
});
