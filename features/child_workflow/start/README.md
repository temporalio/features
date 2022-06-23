# Start child workflow

Child Workflows can be started from a Workflow.

This feature: 

- executes a Workflow that starts a Child Workflow and returns the Child's Workflow Id and `firstExecutionRunId`
- gets the result of the Child Workflow

# Detailed spec

Worker sends the start command to the Server, which creates these two events:

```
WorkflowTaskCompleted
StartChildWorkflowExecutionInitiated
```

and then responds to the Worker, and then the start call completes.