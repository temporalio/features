# Generic codec

Payloads can be run through a custom `my-encoding` Payload Codec.

Steps:

- run an echo workflow that accepts and returns the JSON value `{ "spec": true }`
- verify client result is object `{ "spec": true }`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is `my-encoding`, decode data to extract an inner
payload
- check that the inner payload encoding is `json/plain`, unmarshall its data
using a `json` library, and compare it to the client result
- get argument payload of WorkflowExecutionStarted event from workflow history
- verify that argument and result payloads are the same


# Detailed spec

`metadata.encoding = toBinary("my-encoding")`
`data = [json payload]`
