# Signals can be sent to a workflow execution from non-workflow code

Signals can be sent to a workflow execution from any gRPC client,
not necessarily another workflow.

# Detailed spec

The gRPC `WorkflowService` exposes an rpc call, `SignalWorkflowExecution`, which
can be invoked from code external to a Temporal system. If the RPC invocation
completes successfully, the system guarantees that the signal will be delivered
to the referenced workflow execution. An error is returned if the specified
workflow execution does not exist.
