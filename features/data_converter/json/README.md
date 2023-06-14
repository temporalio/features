# JSON payload converter

JSON values can be converted to and from `json/plain` Payloads.

Steps:

- run an echo workflow that accepts and returns the JSON value
`{ "spec": true }`
- verify client result is object `{ "spec": true }`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is `json/plain`, unmarshall its data using a
`json` library, and compare it to the client result
- get argument payload of WorkflowExecutionStarted event from workflow history
- verify that argument and result payloads are the same


# Detailed spec

`metadata.encoding = toBinary("json/plain")`

- If JSON encoding fails, JsonPayloadConverter returns null/undefined. Since it's the last converter in
  CompositePayloadConverter, CompositePayloadConverter.toPayload throws.
