# Signal-with-start from a workflow: payloads, memo, and visibility

A workflow calls `workflow.signal_with_start_workflow(...)` passing a workflow
input argument, a signal argument, and a memo. This exercises the full
payload-serialization path through the `__temporal_system` Nexus endpoint
(covering what the proto-binary server test verifies, since Python encodes these
payloads automatically).

Verifies: both the caller and the started target complete; the target returns the
signal value; and the memo passed in the request is visible on the target.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and Nexus enabled (`system.enableNexus=true`) with the
built-in `__temporal_system` endpoint.
