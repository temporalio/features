# Nexus async workflow operation succeeds

A workflow invokes a Nexus operation backed by another workflow and receives its result.

# Detailed spec

- A Nexus service with a workflow-run operation is registered on the worker.
- The caller workflow executes the operation against a Nexus endpoint and awaits the result.
- The operation starts a handler workflow; the handler workflow's result is returned as the
  operation result and then as the caller workflow's result.
- An async operation transitions Scheduled -> Started -> Completed.
- The caller's NexusOperationStarted event links to the handler workflow's
  WorkflowExecutionStarted event, and the handler workflow's WorkflowExecutionStarted
  event links back to the caller's NexusOperationScheduled event.
