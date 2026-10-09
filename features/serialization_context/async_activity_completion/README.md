# Serialization context: async activity completion

A client completing an activity out of band has no task to derive the activity
context from, so it has to be given one through the `*WithOptions` client calls.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run an activity that returns `ErrResultPending` after publishing its identity
- heartbeat and complete it with `RecordActivityHeartbeatByIDWithOptions` and
  `CompleteActivityByIDWithOptions`, passing workflow ID, workflow type,
  activity type and task queue
- verify the client result, which requires the workflow to decode the result
  under the very same activity context
- verify that the `ActivityTaskCompleted` result payload carries the activity
  signature

The plain `CompleteActivityByID` has no activity metadata to build a context
from, so a context aware codec must use the `*WithOptions` variants.

Python only puts an activity ID in the workflow side context when the
workflow sets one explicitly, so the workflow schedules the activity with an
explicit activity ID.
