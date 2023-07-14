# Binary payload converter

Binary values can be converted to and from `binary/plain` Payloads.

Steps:

- run a echo workflow that accepts and returns binary value `0xdeadbeef`
- verify client result is binary `0xdeadbeef`
- get result payload of WorkflowExecutionCompleted event from workflow history
- load JSON payload from `./payload.json` and compare it to result payload
- get argument payload of WorkflowExecutionStarted event from workflow history
- verify that argument and result payloads are the same


# Detailed spec

`metadata.encoding = toBinary("binary/plain")`
