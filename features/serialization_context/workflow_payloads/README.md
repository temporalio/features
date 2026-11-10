# Serialization context: workflow payloads

Every workflow scoped payload is converted with a `WorkflowSerializationContext`
carrying the namespace and the workflow ID.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- start a workflow with an input, a memo, a side effect, a signal, a query and
  an update
- verify the client result
- verify that the `WorkflowExecutionStarted` input and memo payloads, the
  `WorkflowExecutionCompleted` result payload, the `WorkflowExecutionSignaled`
  input payload, the `SideEffect` marker data payload, the accepted update input
  payloads and the update outcome payload all carry the workflow signature

Query input and result never reach history; they are covered by the codec, which
fails the query if it was encoded under a different context.
