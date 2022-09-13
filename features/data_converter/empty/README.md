# Null payload converter

Null or undefined values can be converted to and from `binary/null` Payloads.

Steps:

- run a workflow that returns no value
- the workflow call an activity with a null parameter
- activity verifies it recieved a null parameter
- verify client result is null
- get result payload of ActivityTaskScheduled event from workflow history
- load JSON payload from `./payload.json` and compare it to result payload

Note: we don't check the WorkflowExecutionCompleted event payload as well because some SDKs (go, java) skip serializing it.

# Detailed spec

`metadata.encoding = toBinary("binary/plain")`