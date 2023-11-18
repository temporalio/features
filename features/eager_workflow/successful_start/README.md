# Successful start of a workflow using eager mode
Eager Workflow Start (EWS) is a latency optimization to reduce the time to start processing the first task of a workflow. The starter program provisions a slot in a suitable worker, requests that the server starts a workflow in eager mode, and then, when it receives the first WFT in the response, it directly schedules the task to the worker, eliminating a network roundtrip and a database transaction.

In each scenario, the starter program and the worker should share a client. The starter program will create a simple workflow in eager mode, and then verify that eager mode was actually used, and the first workflow task was processed correctly.

# Detailed spec
* The `EnableEagerStart` start workflow option should be `true`.
* The server response to start workflow should include a non-nil `eager_workflow_task` field.
* The task timeout for the workflow should be large enough to hang the program on a task retry.
* The simple workflow should return `"Hello World"` and exit without errors.
