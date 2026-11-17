# Nexus synchronous operation reports cancellation

A synchronous Nexus handler explicitly reports a canceled operation without the caller
requesting cancellation. The caller handles the cancellation and completes successfully.

# Detailed spec

- The caller receives a Nexus operation error with the expected endpoint, service,
  operation, positive scheduled event ID, and empty operation token.
- The immediate cause is a cancellation error with the handler's cancellation message
  and no cancellation details.
- The cancellation error's immediate application cause preserves its type, message,
  retryability flag, and serialized detail, with no additional cause.
- History contains exactly one scheduled and one canceled Nexus event, with no started,
  failed, completed, or cancel-requested event.
- The caller's history is replayed with the same error assertions.
