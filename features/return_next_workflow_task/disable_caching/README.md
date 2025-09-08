# Return next workflow task when caching is disabled

By default, the server is configured to return the next workflow task as part of the workflow task completed response. The SDK can set an option to ask the server not to return the next task. It is used when caching is disabled. We are in the process of deprecating this option and always return the next task. This test ensures the SDK can handle it.

# Detailed spec

- Start a worker with max workflow cache size set to 0. This effectively sets the return_next_workflow_task
to false.
- Run the workflow and check it succeeds successfully.
