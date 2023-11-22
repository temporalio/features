# Successful start of a workflow using eager mode
Eager Workflow Start (EWS) is a latency optimization to reduce the time to start processing the first task of a workflow. The starter program provisions a slot in a suitable worker, requests that the server starts a workflow in eager mode, and then, when it receives the first WFT in the response, it directly schedules the task to the worker, eliminating a network roundtrip and a database transaction.

In each scenario, the starter program and the worker should share a client. The starter program will create a simple workflow in eager mode, and then verify that eager mode was actually used, and the first workflow task was processed correctly.

# Detailed spec
* The `EnableEagerStart` start workflow option should be `true`.
* The server response to start workflow should include a non-nil `eager_workflow_task` field.
* The task timeout for the workflow should be large enough to hang the program on a task retry. A server response with an `eager_workflow_task` alone does not guarantee eager execution because the worker could still refuse to process it. In that exceptional case the task would be retried through the non-eager path, and may succeed. A large timeout effectively disables retries, ensuring success always comes from the eager path.
* The simple workflow should return `"Hello World"` and exit without errors.
