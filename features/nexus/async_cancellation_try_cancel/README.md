# Nexus cancellation returns before delivery

A workflow cancels a started workflow-backed operation using `TRY_CANCEL`. The result
future reports local cancellation without waiting for delivery or operation completion.

# Detailed spec

- The caller waits for a non-empty operation token before canceling its scope.
- A checkpoint activity supplies a workflow task to process the recorded cancellation
  request while its delivery remains gated.
- Only after the result future resolves does a second activity open the cancellation
  handler's gate. Activities keep coordination independent of Nexus request dispatch.
- Go returns a leaf cancellation error directly. Java returns a Nexus operation failure
  with a leaf cancellation cause, correct endpoint/service/operation/scheduled event ID,
  and an empty token for the locally generated failure.
- Waiting for acknowledgement or completion would deadlock the caller before it could
  open the gate.
- A synchronous `release` operation waits for delivery and for the backing workflow to close as
  canceled. All access to the backing workflow stays inside the Nexus service.
- History records the cancellation request before scheduling release. Any cancellation
  acknowledgement or operation-canceled event must follow the gate-opening activity
  being scheduled.
- The release operation completes successfully, and no operation fails or times out.
  The caller's history and the closed backing workflow's history are replayed.
- Synchronization uses channels/latches, not polling or sleeps. Timeouts only bound
  a broken test; there are no assertions about elapsed time.
