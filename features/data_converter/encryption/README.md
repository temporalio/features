# Encryption codec

Payloads can be run through a `binary/encrypted` encryption Payload Codec that uses AES GCM with a 256-bit key.

Steps:

- run a workflow that returns the JSON value `{ "spec": true }`
- verify client result is object `{ "spec": true }`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is "binary/encrypted", decrypt data to extract an
inner payload
- check that the inner payload encoding is `json/plain`, unmarshall its data
using a `json` library, and compare it to the client result

# Detailed spec

`metadata.encoding = toBinary("binary/encrypted")`
`data = [json payload]`
