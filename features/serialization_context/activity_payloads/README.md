# Serialization context: activity payloads

Activity payloads are converted with an `ActivitySerializationContext` carrying
the namespace, workflow ID, workflow type, activity type, task queue, and
`IsLocal = false`.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run an activity that heartbeats and fails its first attempt, so the second
  attempt has to decode the heartbeat details recorded by the first one
- verify the client result
- verify that the `ActivityTaskScheduled` input payload and the
  `ActivityTaskCompleted` result payload carry the activity signature
- verify that the `WorkflowExecutionCompleted` result payload carries the
  workflow signature, not the activity one

Python only puts an activity ID in the workflow side context when the
workflow sets one explicitly, so the workflow schedules the activity with an
explicit activity ID.
