# Child workflows throw on timeout

Executing a Child Workflow throws if the Child times out.

This feature: 

- executes a Child Workflow with `workflowExecutionTimeout: '1ms'` and `retry: { maximumAttempts: 1 }`
- verifies that the execute throws