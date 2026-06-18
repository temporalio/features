# Standalone Nexus operation succeeds

A client starts and awaits an async, workflow-backed Nexus operation directly, without a caller
workflow.

# Detailed spec

- A Nexus service with an async, workflow-run operation is registered on the worker.
- The test client builds a standalone `NexusClient` from `client.Client.NewNexusClient` and
  calls `ExecuteOperation` against a Nexus endpoint.
- The operation starts asynchronously, backed by a handler workflow; the handler workflow's result
  is returned via the operation handle.
- The handle's `Get` blocks until the handler workflow completes and returns the operation result
  without any caller workflow being involved.

# Supported SDKs

- Go: requires SDK v1.44.0 or later (the standalone Nexus operation client API is Go-only at this
  time).
