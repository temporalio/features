# Retry child workflow

A Child Workflow can have a Retry Policy.

This feature:

- executes with `maxAttempts: 3` a Child Workflow that always fails
- verifies the Child was executed 3 times

# Detailed spec

TODO