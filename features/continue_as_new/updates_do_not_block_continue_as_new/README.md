# updates_do_not_block_continue_as_new

Workflows are subject to a maximum history size. As a result, very long running or
infinite lifetime workflows require some mechanism to avoid running into this limit.
Continue as new provides that facility by allowing a workflow to complete and pass
state to a new execution of the same workflow.

# Detailed spec

* The client starts a workflow that will continue as new (CAN).
* The client sends an update in such a way that it is guaranteed that it is admitted by the server while the WFT is in flight.
* The workflow responds to the WFT with a ContinueAsNewWorkflowExecution command.
* The workflow handles the update in a way that returns information to the caller about the run on which it was handled.
* Verify that the update was handled on the post-CAN run.
