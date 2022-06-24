# Local Activity - Non Retryable Error Types in Retry Policy

When a Workflow schedules a Local Activity, it can configure the retry policy for that invocation
via the Local Activity Options.

One of the retry policy attributes is a list of non retryable error types. A Local Activity is not
retried if it fails with an error with a type supplied in that list.

# Detailed spec
* Local activity options should accept a list of error types that will not be retried for that
  LA invocation
* When such an error is thrown/returned, the semantics match those of directly throwing/returning a 
  [non-retryable error](../non_retryable_error_from_activity/README.md)

