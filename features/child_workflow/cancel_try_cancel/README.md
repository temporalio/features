# Child workflow cancel with `TRY_CANCEL`

Child Workflows can be cancelled with `TRY_CANCEL` ChildWorkflowCancellationType.

This feature:

- starts a Child Workflow with `TRY_CANCEL`
- cancels the start context
- verifies that:
  - start throws immediately
  - the Child receives a cancellation request