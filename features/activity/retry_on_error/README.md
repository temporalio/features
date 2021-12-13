# Retrying activities on error

In Temporal, failed activities can retry in a number of ways. This is configurable by retry policies that govern if and
how a failed activity may retry.

This feature contains an activity that always fails. It is started with a retry policy that does not backoff and only
retries 4 times for a total attempt count of 5. It is then confirmed that only 5 attempts were made before the activity
error bubbled up through the workflow.