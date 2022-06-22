# Local Activity - Non Retryable Error Types in Retry Policy

When a Workflow schedules a Local Activity, it can configure the retry policy for that invocation
via the Local Activity Options.

One of the retry policy attributes is a list of non retryable error types. A Local Activity is not
retried if it fails with an error with a type supplied in that list.
