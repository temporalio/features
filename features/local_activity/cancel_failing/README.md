# Cancel a failing Local Activity

A backing off Local Activity (between a failed and next attempt) can be cancelled, the Workflow
perceives the Activity is cancelled and the appropriate marker is recorded in history.

# Detailed spec
* If the backing off activities are backing off locally, rather than using a timer
  (see [that spec](../backoff_with_persistent_timer/README.md)), the local backoff
  timer is cancelled, and the SDK will issue a `RecordMarker` command indicating that
  the local activity is cancelled
* If there is a server-side timer scheduled, then the SDK will issue a `CancelTimer`
  command along with a `RecordMarker` command indicating that the local activity is cancelled