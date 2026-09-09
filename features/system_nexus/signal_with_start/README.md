# Workflow-side signal with start

A workflow can signal another workflow and start it atomically when it is not
already running. This experimental API is implemented as a System Nexus
operation.

The basic execution starts a target workflow with its first signal, then invokes
the operation again for the same workflow ID. It verifies that the target
receives the start input and both signals, proving that the second operation
used the existing execution.

A separate execution uses a context-aware payload converter and codec. It
verifies that Signal-with-Start inner payloads have the target workflow
serialization context and pass through the codec; the codec rejects the outer
System Nexus envelope if it is passed to it. Python also performs this execution
with an in-memory external-storage driver and verifies that inner payloads are
externalized while the outer envelope is not.
