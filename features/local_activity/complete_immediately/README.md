# Local Activity completes immediately

When a Local Activity completes immediately after being scheduled (in the same Workflow Task), the
result is delivered back to the Workflow.

Assuming the Workflow does not schedule any more Local Activities in this Workflow Task, the task
completes and a Local Activity marker is recorded in history.

# Detailed spec
* When an LA completes in the same WFT it was started in, a `RecordMarker` command is included in
  the next WFT completion with the result.
