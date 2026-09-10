# Nexus async operation succeeds

A workflow invokes an asynchronous Nexus operation backed by a workflow run, observes the
operation token, and then receives the backing workflow's result.

# Detailed spec

- A Nexus service with a workflow-run operation is registered on the worker, along with the
  backing workflow it starts.
- The caller workflow executes the operation and first awaits the operation execution, which
  carries a non-empty operation token.
- The caller then awaits the operation result, which is the output of the backing workflow.
- An async operation is scheduled, then started once the backing workflow is running, and
  completed when the backing workflow completes.
