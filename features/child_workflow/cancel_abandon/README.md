# Child workflow cancel with `ABANDON`

Child Workflows can be cancelled with `ABANDON` ChildWorkflowCancellationType.

This feature:

- starts a Child Workflow with `ABANDON`
- cancels the start context
- verifies that:
  - start throws immediately
  - the Child does not receive a cancellation request

# Detailed spec

TODO