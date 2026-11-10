# Serialization context: continue-as-new payloads

Continue-as-new keeps the workflow ID, so the arguments handed to the next run
are converted with the same `WorkflowSerializationContext`.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run a workflow that continues as new once
- verify the client result
- verify that the first run's `WorkflowExecutionContinuedAsNew` input payload,
  the last run's `WorkflowExecutionStarted` input payload and its
  `WorkflowExecutionCompleted` result payload all carry the same workflow
  signature
