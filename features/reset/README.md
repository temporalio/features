# Workflow resetting

Workflows may be "reset" to a previous point in history by calling the `ResetWorkflowExecution`
RPC method. This causes any outstanding workflow tasks to be invalidated, a new workflow
execution is created with the appropriately truncated history. The workflow ID will be
the same, but the workflow will have a new run ID (as well as knowledge of the original run id).
Workflow execution then proceeds normally.

# Detailed spec

TODO: Could use input from server folks

* SDK implementations must respect the fact that a workflow's run id may appear to
  change partway through execution as a result of a reset. The new run id is provided
  in a Workflow Task Failed event which will be present in the history of the new
  workflow. This run id should be used from then on for randomness seeding.

