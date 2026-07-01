# Signal-with-start from a workflow: terminated target

A target workflow is started and then terminated (closed). A workflow then calls
`workflow.signal_with_start_workflow(...)` for the same workflow id. Because the
previous run is closed, a fresh run is started.

Verifies: the returned run id differs from the terminated run's run id (a new
execution was started).

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint.
