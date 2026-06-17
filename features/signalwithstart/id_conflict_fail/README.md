# Signal-with-start from a workflow: FAIL conflict policy is rejected

A workflow calls `workflow.signal_with_start_workflow(...)` with
`id_conflict_policy=FAIL`. This policy is not supported for signal-with-start, so
the server rejects the scheduled Nexus operation. The rejection surfaces as a
workflow task failure (not a catchable error), so the caller workflow never
completes.

Verifies: the caller's history contains a workflow task failure whose message
indicates the conflict policy is not supported.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and Nexus enabled (`system.enableNexus=true`) with the
built-in `__temporal_system` endpoint.
