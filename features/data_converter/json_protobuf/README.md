# JSON protobuf payload converter

Protobuf values can be converted to and from `json/protobuf` Payloads.

Steps:

- run an echo workflow that accepts and returns a
[`DataBlob`](https://pkg.go.dev/go.temporal.io/api/common/v1#DataBlob)
with binary value `0xdeadbeef`
- verify client result is [`DataBlob`] with value `0xdeadbeef`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is `json/protobuf`, unmarshall its data into a
`DataBlob` using `protojson` library, and compare it to the client
result
- get argument payload of WorkflowExecutionStarted event from workflow history
- verify that argument and result payloads are the same


# Detailed spec

`metadata.encoding = toBinary("json/protobuf")`
`metadata.messageType = toBinary("temporal.api.common.v1.DataBlob")` (used by languages that cannot get a parameter's type at runtime)

Rust SDK 0.7 does not provide a `json/protobuf` payload converter. The Rust feature therefore
uses a minimal local converter and verifies its history payload against `payload.json`, an
independent fixture for the Temporal `json/protobuf` wire contract.
