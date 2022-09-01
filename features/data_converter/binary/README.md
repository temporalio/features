# Binary payload converter

Binary values can be converted to and from `binary/plain` Payloads.

Steps:

- run a workflow that returns binary value `0xdeadbeef`
- verify client result is binary `0xdeadbeef`
- get result payload of WorkflowExecutionCompleted event from workflow history
- load JSON payload from `./payload.json` and compare it to result payload

# Detailed spec

`metadata.encoding = toBinary("binary/plain")`
