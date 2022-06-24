# Concurrent Local Activities

A Workflow can schedule concurrent Local Activities in the same Workflow Task.

When all Local Activities are resolved, the task should be completed and a marker will be present
for each completed Local Activity.

On the Workflow side, the result of all Activities will be available.

# Detailed spec
* Local activities may execute concurrently. The semantics match those for completing
  [eventually](../complete_eventually/README.md) or [immediately](../complete_immediately/README.md)
* If some activities complete while others are running, their `RecordMarker` commands will be issued
  on the next WFT heartbeat.