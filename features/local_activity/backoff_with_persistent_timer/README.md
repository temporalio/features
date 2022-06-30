# Backoff with persistent timer

When a Local Activity fails for longer than `ActivityOptions.localRetryThreshold`, the SDK uses a
server side (persistent) timer to backoff until the next attempt.

# Detailed spec
* When the local threshold is exceeded, the SDK will send a `RecordMarker` command indicating that
  the local activity failed. The marker should contain data indicating that the activity is about to
  backoff, and for how long.
* In the same WFT completion, the SDK will send a `StartTimer` command whose duration is equal to
  the backoff.
* Once the timer fires, the SDK will execute the local activity again with identical values but
  a next sequence number, and an appropriate value for `attempts`.