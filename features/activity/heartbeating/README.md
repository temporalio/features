# Activity heartbeating
Activities can (and typically should) heartbeat periodically. When they do, they can
attach details, which enables remembering progress in the event that the activity worker
crashes or similar. Activities may be scheduled with a heartbeat timeout value, and if
they do not heartbeat at least that frequently, they will be considered timed out.
Additionally, activities are only notified of cancellation requests via heartbeating responses.


# Detailed spec
* Activities, while running, may call an SDK-provided function for heartbeating which corresponds
  to calling the RecordActivityTaskHeartbeat API
* Each call to the SDK API does not necessarily equal one API call, because SDKs implement
  throttling of the calls. This allows the user to heartbeat in a tight loop without sending too
  many rpc requests