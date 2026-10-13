# Serialization context: child workflow payloads (generated ID)

When a child workflow is started **without** an explicit workflow ID, the SDK
assigns one deterministically. The child's payloads must be converted with that
generated child ID, not with the parent's ID.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run a workflow that starts a child workflow **without** a workflow ID
- verify the client result
- discover the generated child workflow ID from the parent's
  `ChildWorkflowExecutionStarted` event and confirm it differs from the parent's
- verify that the parent's `StartChildWorkflowExecutionInitiated` input payload
  and `ChildWorkflowExecutionCompleted` result payload carry the child's
  signature
- verify that the child's `WorkflowExecutionStarted` input payload carries the
  same signature
