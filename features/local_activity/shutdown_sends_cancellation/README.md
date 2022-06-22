# Local Activity gets cancelled when a Worker is shutting down

When a Worker is shutting down, it cancels any running Local Activities.

Assuming the Local Activity listens for and acknowledges cancellation, it should remain unresolved and
the current Workflow Task should fail so it could immediately be retried in another Worker.
