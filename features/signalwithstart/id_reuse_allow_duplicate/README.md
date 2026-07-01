# Signal-with-start from a workflow: ALLOW_DUPLICATE reuse policy

A target workflow id has a previously-completed run. A workflow calls
`workflow.signal_with_start_workflow(...)` with `id_reuse_policy=ALLOW_DUPLICATE`.
Because duplicates are allowed for closed runs, a new execution is started.

Verifies: the returned run id differs from the prior completed run's run id.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint.
