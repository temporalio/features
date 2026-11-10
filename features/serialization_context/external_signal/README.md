# Serialization context: external signal payloads

A signal sent to another workflow is converted with the *target's* workflow ID.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- start a receiver workflow that waits for a signal
- run a workflow that signals the receiver through
  `SignalExternalWorkflow`
- verify that the sender's `SignalExternalWorkflowExecutionInitiated` input
  payload and the receiver's `WorkflowExecutionSignaled` input payload carry the
  receiver's signature
