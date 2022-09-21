# signals_block_completion

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
* If (this is the case for any execution-completing command) the server has received new signals
  for the workflow while the WFT was being processed, the WFT must be retried.
* Users should be aware that they may want to ensure signal channels are drained before
  continuing as new, if the language (Go) doesn't use explicit handlers.