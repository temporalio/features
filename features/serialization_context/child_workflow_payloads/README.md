# Serialization context: child workflow payloads

Child workflow payloads are converted with the *child's* workflow ID, not the
parent's.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run a workflow that starts a child workflow with an explicit workflow ID
- verify the client result
- verify that the parent's `StartChildWorkflowExecutionInitiated` input payload
  and `ChildWorkflowExecutionCompleted` result payload carry the child's
  signature
- verify that the child's `WorkflowExecutionStarted` input payload carries the
  same signature
