# Nexus sync operation succeeds

A workflow invokes a synchronous Nexus operation and receives its result.

# Detailed spec

- A Nexus service with a sync operation is registered on the worker.
- The workflow executes the operation against a Nexus endpoint and awaits the result.
- The operation completes synchronously and its output is returned as the workflow result.
- A sync operation never enters the started state — it transitions directly from scheduled
  to completed.
