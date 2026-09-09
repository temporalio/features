# System Nexus

System Nexus operations are ordinary Nexus operations that target the reserved
`__temporal_system` endpoint. The Temporal server handles them directly rather
than routing them to a user-defined Nexus handler.

## Serialization

The System Nexus operation input (the *System Nexus envelope*) must remain
readable by the server. SDKs therefore cannot apply their user data converter
to the envelope as a whole: its outer representation must reach the server as
binary protobuf. The encoded envelope is marked with
`__temporal_system_payload` metadata so SDK infrastructure can recognize it in
later processing.

### Inner payloads

Payloads contained within the envelope—for example, Signal-with-Start input
arguments, memo, headers, and search attributes—must use the application's
data converter so the target Workflow can consume them.

When an SDK processes payloads, it recognizes a System Nexus envelope by its
payload marker. Instead of applying its operation to the outer envelope, it
deserializes the envelope, applies generic payload visitation to all nested
application payloads, and serializes the envelope again. This applies to
payload codecs and external storage today.

### Serialization context

Each System Nexus operation can require a different serialization context, and
that context can differ from an ordinary Nexus operation's context. For
example, Signal-with-Start's nested payloads require the target Workflow's
serialization context so the target can decode them.

SDKs derive the context from the System Nexus operation through generated code.
The current model is an operation annotation naming a function that receives
the operation input and returns a serialization context.

## Interception

Each System Nexus operation has two interception points with distinct purposes.

The operation-specific interception point receives the complete generated
request type. It exposes the arguments, headers, and other values delivered to
the target and is the appropriate point for tracing propagation and most
operation-specific interception behavior.

The generic System Nexus interception point receives the outer Nexus-operation
scheduling input. It is functionally equivalent to the ordinary generic Nexus
interception point but permits policies that apply to all System Nexus
operations. For example, an authentication proxy may require headers that do
not belong on the server-facing System Nexus request.

## Language-specific considerations

### TypeScript

In TypeScript, creating the protobuf-binary envelope inside the Workflow
isolate is difficult because protobufjs requires writes. The isolate therefore
serializes the envelope with the normal JSON converter. Worker-side payload
processing converts it to protobuf binary before it reaches the server. This
conversion happens regardless of whether a payload codec or external storage
is configured.

The serialization-context factory is defined for the user model type, while
the transfer type is what remains after leaving the isolate. TypeScript
therefore stores the derived context in `__temporal_system_context` metadata on
the JSON envelope so it can be reused outside the isolate.

## Adding an operation

Adding a System Nexus operation requires generated transfer types, a
serialization-context factory, a discoverable protobuf service and method
descriptor, and payload-visitor coverage for every nested payload field.
