# Signal-with-start from a workflow: start delay

A workflow calls `workflow.signal_with_start_workflow(...)` with a `start_delay`.
The target is scheduled to start after the delay; the signal is buffered and
delivered when the run begins.

Verifies: a run id is returned, and after the delay the target starts, receives
the buffered signal, and completes returning the signal value.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint.
