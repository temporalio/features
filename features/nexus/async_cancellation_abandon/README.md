# Nexus cancellation abandons the operation

A workflow cancels a started workflow-backed operation using `ABANDON`. Its result
future reports local cancellation without requesting cancellation of the operation.

# Detailed spec

- The caller waits for a non-empty operation token before canceling its scope.
- The result reports local cancellation. Go returns a leaf cancellation error directly;
  Java returns a Nexus operation failure with a leaf cancellation cause, correct
  endpoint/service/operation/scheduled event ID, and an empty token for the locally
  generated failure.
- The backing workflow remains running. The caller invokes a synchronous `release`
  operation, which signals the backing workflow and returns its successful result.
  If abandonment had canceled the backing workflow, this could not succeed.
- History contains no cancellation request, acknowledgement, canceled, failed, or
  timed-out Nexus event.
- The abandoned operation's completion callback may arrive before the caller closes,
  but the test does not depend on callback timing. The release operation must complete.
- All backing-workflow access stays inside the Nexus service. There is no polling,
  sleeping, or cross-namespace inspection. Both workflow histories are replayed.
