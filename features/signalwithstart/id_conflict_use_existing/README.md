# Signal-with-start from a workflow: USE_EXISTING conflict policy

A target workflow is already running. A workflow calls
`workflow.signal_with_start_workflow(...)` with
`id_conflict_policy=USE_EXISTING`. The existing run is signaled and its run id is
returned; no new run is started.

Verifies: the returned run id equals the already-running target's run id.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and Nexus enabled (`system.enableNexus=true`) with the
built-in `__temporal_system` endpoint.
