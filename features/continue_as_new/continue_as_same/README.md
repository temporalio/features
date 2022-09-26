# Continue as same

Workflows are subject to a maximum history size. As a result, very long running or
infinite lifetime workflows require some mechanism to avoid running into this limit.
Continue as new provides that facility by allowing a workflow to complete and pass
state to a new execution of the same workflow.


# Detailed spec

* Workflows may choose to continue as new at any point. Semantically, this is best thought
  of as the workflow returning with a special value indicating it would like to continue.
  However, many SDKs choose to implement this as a free-floating API that may be called anywhere
  in workflow code, or signal handlers, etc.
* When that happens, the next WFT should have a ContinueAsNewWorkflowExecution command
* Workflow options and arguments should be passed to the new WFT unless they were explicitly changed.

## Feature implementation
* Set a test memo in the workflow options
* Execute a workflow that checks if it was started by continue as new
  * If yes, end execution
  * If no, continue as new
* Check the workload returned the correct value
* Check the memos persisted after continue as new
* TODO Check the search attributes persisted after continue as new