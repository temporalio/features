# Backoff with persistent timer

When a Local Activity fails for longer than `ActivityOptions.localRetryThreshold`, the SDK uses a
server side (persistent) timer to backoff until the next attempt.
