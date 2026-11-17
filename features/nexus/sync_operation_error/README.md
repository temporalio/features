# Nexus sync operation fails

A workflow invokes a synchronous Nexus operation that fails with an operation error and
inspects the resulting error.

# Detailed spec

- The handler fails the operation by raising an operation error in the failed state, carrying
  an application error with a known type and message as its cause. Raising an application
  error directly is reported as a handler error instead, which this feature does not cover.
- The caller receives a Nexus operation error with the expected endpoint, service, operation,
  positive scheduled event ID, and empty operation token.
- Its immediate cause is an application error of type `OperationError`, carrying the explicit
  operation failure message. That wrapper's immediate cause is the handler's application error,
  with its original type, message, retryability flag, and serialized detail preserved, and no
  additional cause. Finding a matching error somewhere in the cause chain is not sufficient.
- The caller handles the failure and completes successfully.
- History contains exactly one scheduled and one failed Nexus event, with no started event or
  other Nexus lifecycle events. The caller's history is replayed with the same error assertions.
