# Local Activity completes eventually

Local activities might take longer than one Workflow Task timeout to complete. When they do,
the SDK will perform WFT "heartbeating" in order to avoid running into the timeout while the
activity runs.

When the Local Activity eventually completes, a Local Activity marker is recorded in history, and
the workflow code receives the result.

# Detailed spec
* If there are running local activities after invoking workflow code, the WFT will not be
  completed until either all LAs are complete, or some (configurable) percentage of the WFT timeout
  has elapsed.
* If said threshold was reached, a WFT "heartbeat" is issued, completing the WFT. If the workflow
  code has generated any commands in the meantime, those are included with the completion. There
  may be no commands.
* Once the LA completes, a `RecordMarker` command is included in the next WFT completion with the
  result.