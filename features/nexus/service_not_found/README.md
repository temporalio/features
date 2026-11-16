# Nexus service does not exist

A workflow calls an unregistered Nexus service through a valid endpoint whose worker
has a different service registered. This tests service lookup, not endpoint lookup or
the absence of a Nexus worker.

# Detailed spec

- The caller requests `echo` on `missing-service`. The worker only registers `echo`
  on `test-service`.
- The workflow receives a Nexus operation error with the requested endpoint, service,
  operation, positive scheduled event ID, and empty operation token.
- Its immediate cause is a `NOT_FOUND` handler error, not an application operation
  failure. Retry behavior is unspecified and the error is non-retryable by default.
- The handler diagnostic identifies the missing service. Exact SDK-generated wording
  is not required to match between languages.
- The caller handles the failure and completes successfully.
- History contains exactly one scheduled and one failed Nexus event, with no started
  or completed event. The caller's history is replayed.
