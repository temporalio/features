# Signal with start

One can send a signal that will start a workflow execution if the
specified workflow execution is not already running. In such cases the signal is
immediately delivered to the newly started workflow.

This feature calls the `SignalWithStartWorkflowExecution` API to send a signal
to a workflow execution that does not exist and verifies that (a) the specified
workflow execution is started and (b) the supplied signal is delivered to the
same.

# Detailed spec

Upon receiving a `SignalWithStartWorkflowExecution` the server will look up an
existing workflow execution with the supplied workflow ID and run ID. If that
workflow execution does not exist then it is started. The first event for the
new workflow other than the workflow-started event will be the signal-received
event and consequently the signal will be delivered to the workflow immediately
after it is started. If the referenced workflow already exists, the signal is
delivered as though `SignalWorkflowExecution` were called.
