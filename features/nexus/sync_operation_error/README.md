# Nexus sync operation fails

A workflow invokes a synchronous Nexus operation that fails with an operation error and
inspects the resulting error.

# Detailed spec

- The handler fails the operation by raising an operation error in the failed state, carrying
  an application error with a known type and message as its cause. Raising an application
  error directly is reported as a handler error instead, which this feature does not cover.
- The caller receives a Nexus operation error, and the application error raised by the handler
  is present in its cause chain with the original type and message preserved.
- The caller handles the failure and completes successfully.
- A failed operation produces a failed event and never a completed one.
