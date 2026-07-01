# Signal-with-start from a workflow: signal an existing run

A target workflow is already running. A workflow then calls
`workflow.signal_with_start_workflow(...)` for the same workflow id. Because a run
already exists, the operation signals it rather than starting a new one.

Verifies: the returned run id equals the already-running target's run id (no new
execution was started).

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint.
