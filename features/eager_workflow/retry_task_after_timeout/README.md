# Retry eager workflow task after timeout

When starting a workflow with `request_eager_execution`, if task processing times out, the server eventually deliver the
task to a worker to be retried.

# Detailed spec

- Start a workflow with `request_eager_execution`
- Assert that a workflow task is returned inline
- Intentionally time out the task
- Wait for worker to get the task via polling
- Assert that the started workflow completes
