# Nexus workflow-backed operation fails

A workflow invokes a workflow-backed Nexus operation whose backing workflow fails with a
known application error. The caller handles the failure and completes successfully.

# Detailed spec

- The operation execution resolves successfully with a non-empty token before the caller
  awaits the failed result.
- The result fails with a Nexus operation error carrying the endpoint, service, operation,
  scheduled event ID, and the same operation token.
- Its immediate cause is the backing workflow's application error, with its type, message,
  retryability flag, and serialized detail preserved. There is no workflow-client failure
  wrapper or synchronous `OperationError` wrapper, and no additional cause.
- History contains exactly one scheduled, started, and failed Nexus event, in that order,
  with no other Nexus lifecycle events.
- The caller's history is replayed with the same error assertions.
