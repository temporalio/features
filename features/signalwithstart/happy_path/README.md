# Signal-with-start from a workflow: happy path

A workflow calls `workflow.signal_with_start_workflow(...)` (routed through the
`__temporal_system` Nexus endpoint) to start a brand-new target workflow and
deliver a signal to it in one operation. The caller returns the started target's
run id; the target completes after receiving the signal and returns the signal
value.

Verifies: the operation starts a new execution (non-empty run id) and the signal
is delivered (target returns the signal value).

## Server requirements

This feature requires a server with namespace dynamic config:

- `history.enableSignalWithStartFromWorkflow=true`
- `history.enableChasm=true` (default true)
- Nexus enabled (`system.enableNexus=true`) with the built-in `__temporal_system`
  endpoint available.
