# Retry eager workflow start request after timeout

When starting a workflow with `request_eager_execution`, if the request is retried after workflow task timeout, the
server should respond with an error and prevent eager execution.

# Detailed spec

- Start a workflow with `request_eager_execution`
- Assert that a workflow task is returned inline
- Wait for the workflow task timeout
- Send a second request as a duplicate of the first
- Assert that the server responds with an error
- Wait for worker to get the task via polling
- Assert that the started workflow completes
