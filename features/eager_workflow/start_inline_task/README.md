# Start workflow with eager execution

When starting a workflow with `request_eager_execution`, a task is returned in the start response and can be completed.

# Detailed spec

- Start a workflow with `request_eager_execution`
- Assert that a workflow task is returned inline
- Fabricate a workflow task response with workflow completion (TODO: replace once SDK implements eager tasks)
- Complete the workflow task
- Assert that the completion is accepted
- Get the workflow result and assert correctness
