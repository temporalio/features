# Nexus non-retryable handler error fails the operation

A workflow calls a synchronous Nexus handler that explicitly rejects the request with a
non-retryable handler error. The caller handles that failure and completes successfully.

# Detailed spec

- The handler reports `UNAVAILABLE`, which would normally retry, with an explicit
  non-retryable override and an application error cause.
- The caller receives a Nexus operation error with the expected endpoint, service,
  operation, positive scheduled event ID, and empty operation token.
- Its immediate cause is a handler error, not an application operation error. The handler
  error retains its `UNAVAILABLE` type, explicit message, and non-retryable behavior.
- The handler error's immediate application cause preserves its type, message, retryability
  flag, and serialized detail, with no additional cause.
- History contains exactly one scheduled and one failed Nexus event, with no started event
  or other Nexus lifecycle events. The caller's history is replayed.
