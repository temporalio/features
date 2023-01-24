# Retry eager workflow start request immediately

When starting a workflow with `request_eager_execution`, if the request is retried, an inline task should be delivered
in the server response as long as the task has not been timed out (or made invalid for other reasons).

# Detailed spec

- Start a workflow with `request_eager_execution`
- Assert that a workflow task is returned inline
- Send a second request as a duplicate of the first
- Assert that a workflow task is returned inline
- Complete the workflow task
- Assert that the completion is accepted
- Get the workflow result and assert correctness
