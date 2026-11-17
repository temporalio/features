# Nexus cancellation waits for delivery

A workflow cancels a started workflow-backed operation using `WAIT_REQUESTED`. It
waits for cancellation acknowledgement, but not for the operation to complete.

# Detailed spec

- The caller waits for a non-empty operation token before canceling its scope.
- The cancellation handler is gated. A checkpoint activity provides a workflow task
  with delivery still blocked, and the result future must remain unresolved. A second
  activity then opens the gate. This distinguishes waiting for delivery from `TRY_CANCEL`.
- The backing workflow waits for a release signal in a disconnected cancellation scope.
  Receiving cancellation alone therefore cannot finish the operation.
- The caller's result future reports local cancellation. Go returns a leaf cancellation
  error directly; Java returns a Nexus operation failure with a leaf cancellation cause,
  correct endpoint/service/operation/scheduled event ID, and an empty token for the
  locally generated failure.
- Only after that result resolves does the caller invoke a synchronous `release`
  operation. It signals the backing workflow and waits for it to close as canceled.
- History must record cancellation acknowledgement before release is scheduled.
  Any operation-canceled event must follow release being scheduled.
- These checks distinguish this mode from both returning before delivery and waiting
  for completion. The release operation is the only successful operation completion.
- All backing-workflow access stays inside the Nexus service. There is no polling,
  sleeping, or cross-namespace inspection. Both workflow histories are replayed.
