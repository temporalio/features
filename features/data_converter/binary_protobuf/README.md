# Binary protobuf payload converter

Protobuf values can be converted to and from `binary/protobuf` Payloads.

Steps:

- run a workflow that returns [`DataBlob`](https://pkg.go.dev/go.temporal.io/api/common/v1#DataBlob)
with binary value `0xdeadbeef`
- verify client result is [`DataBlob`] with value `0xdeadbeef`
- get result payload of WorkflowExecutionCompleted event from workflow history
- verify payload encoding is `binary/protobuf`, unmarshall its data into a
`DataBlob` using `protobuf` library, and compare it to the client
result

# Detailed spec

`metadata.encoding = toBinary("binary/protobuf")`
`metadata.messageType = toBinary("DataBlob")` (used by languages that cannot get a parameter's type at runtime)
