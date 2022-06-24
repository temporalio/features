# Non Retryable Error thrown from a Local Activity

A Local Activity is not retried if it fails with a non retryable ApplicationFailure.

# Detailed spec
* The SDK will resolve the LA as failed and issue a `RecordMarker` command with the failure details.