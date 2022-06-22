# Child workflow cancel with `WAIT_COMPLETED`

Child Workflows can be cancelled with `WAIT_COMPLETED` ChildWorkflowCancellationType.

This feature:

- starts a Child Workflow with `WAIT_COMPLETED`
- cancels the start context
- verifies that:
  - the Child ignores cancellatiion
  - start doesn't throw

# Detailed spec

TODO