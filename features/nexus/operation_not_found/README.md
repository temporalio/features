# Nexus operation does not exist

A workflow calls an unregistered operation on a registered Nexus service through a valid
endpoint. This tests operation lookup separately from service and endpoint lookup.

# Detailed spec

- The worker registers `test-service` with an `echo` operation, but the workflow requests
  `missing-operation` on that same service.
- The workflow receives a Nexus operation error with the requested endpoint, service,
  operation, positive scheduled event ID, and empty operation token.
- Its immediate cause is a `NOT_FOUND` handler error, not an application operation
  failure. Retry behavior is unspecified and the error is non-retryable by default.
- The handler diagnostic identifies the missing operation. Exact SDK-generated wording
  is not required to match between languages.
- The caller handles the failure and completes successfully.
- History contains exactly one scheduled and one failed Nexus event, with no started
  or completed event. The caller's history is replayed.
