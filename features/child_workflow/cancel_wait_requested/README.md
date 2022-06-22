# Child workflow cancel with `WAIT_REQUESTED`

Child Workflows can be cancelled with `WAIT_REQUESTED` ChildWorkflowCancellationType.

This feature:

- starts a Child Workflow with `WAIT_REQUESTED`
- cancels the start context
- verifies that:
  - start throws once the server receives the cancellation request
  - the Child then receives a cancellation request