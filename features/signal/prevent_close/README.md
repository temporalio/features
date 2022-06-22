# Pending signals prevents workflows from closing

Any pending signal for a given workflow execution will prevent that
workflow execution from completing.

This feature tests that a signal queued for processing at the time that a
workflow would otherwise exit in fact keeps the workflow running until the
signal has been delivered and processed.

# Detailed spec

In most cases, having run the workflow function to completion, the SDK will send
a `RespondWorkflowTaskCompletedRequest` to the server containing a command of
type `COMMAND_TYPE_COMPLETE_WORKFLOW_EXECUTION` to indicate that the workflow
execution should complete. However, if the server has a pending signal it must
refuse to end the workflow execution and return an error from the
`RespondWorkflowTaskComplete` call. The SDK will evict the workflow and replay
it, finding and delivering the pending signal in the process.
