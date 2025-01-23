# signals_block_continue_as_new

Workflows are subject to a maximum history size. As a result, very long running or
infinite lifetime workflows require some mechanism to avoid running into this limit.
Continue as new provides that facility by allowing a workflow to complete and pass
state to a new execution of the same workflow.

* Workflows may choose to continue as new at any point. Semantically, this is best thought
  of as the workflow returning with a special value indicating it would like to continue.
  However, many SDKs choose to implement this as a free-floating API that may be called anywhere
  in workflow code, or signal handlers, etc.
* When that happens, the next WFT response should have a ContinueAsNewWorkflowExecution command
* If the server has received new signals for the workflow while the WFT was being processed, the WFT must be
  retried. Note that this is the case for any execution-completing command; not just continue as new.
* Users should be aware that they may want to ensure signal channels are drained before
  continuing as new, if the language (Go) doesn't use explicit handlers.


# Detailed spec

* The client starts a workflow that will continue as new (CAN).
* The client sends a signal in such a way that it is guaranteed that it is made durable by the server while the WFT is in flight.
* The workflow responds to the WFT with a ContinueAsNewWorkflowExecution command.
* Verify that the WFT is retried and that the signal is handled on the pre-CAN run.
