# Compression codec

Payloads can be run through a `binary/zlib` compression Payload Codec.

Steps:

- run a workflow that returns the JSON value `{ "spec": true }`
- verify client result is object `{ "spec": true }`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is "binary/zlib", uncompress data to extract an inner
payload
- check that the inner payload encoding is `json/plain`, unmarshall its data
using a `json` library, and compare it to the client result

# Detailed spec

`metadata.encoding = toBinary("binary/zlib")`
`data = [json payload]`
