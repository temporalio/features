# Signal-with-start from a workflow: REJECT_DUPLICATE reuse policy

A target workflow id has a previously-completed run. A workflow calls
`workflow.signal_with_start_workflow(...)` with `id_reuse_policy=REJECT_DUPLICATE`.
The operation is rejected because a closed run with that id already exists.

Verifies: the signal-with-start operation fails (the caller captures the failure,
whose cause message indicates a duplicate/already-started workflow).

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint. Also set
`system.workflowIdReuseMinimalInterval=0` to avoid reuse-interval throttling.
