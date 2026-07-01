# Signal-with-start from a workflow: TERMINATE_EXISTING conflict policy

A target workflow is already running. A workflow calls
`workflow.signal_with_start_workflow(...)` with
`id_conflict_policy=TERMINATE_EXISTING`. The running run is terminated and a new
run is started.

Verifies: the returned run id differs from the original, and the original run's
status is TERMINATED.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint.
