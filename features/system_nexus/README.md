# System Nexus

System Nexus operations are normal nexus operations which happen to target the `__temporal_system` endpoint. They are handled by the server rather than a user's own defined nexus handler.

## Serialization

Because the operations need to be read by the server, the nexus operation's input, which we'll call the "system nexus envelope" needs to be handled uniquely. It can't be serialized with the user's data converter (payload converter, codec, or external storage) because the server would then be unable to read it. Instead, it needs to reach the server specifically encoded with binary protobuf. It should additionally have `__temporal_system_payload` metadata for use in later recognition.

### Inner Payloads

The payloads which are contained within the envelope however, like the input arguments, memos, etc in SignalWithStart, do need to use the user's dataconverter so that they are usable when they reach the target workflow. To accomplish this, anything that processes payloads needs to be updated to handle the recognition and handling of system nexus envelopes. This includes codec application and external storage today. Upon reaching a system nexus envelope, the code (usually a generic visitor for code sharing) should recognize a system nexus envelope by its payload marker metadata. Instead of applying the current operation to it, it deserializes that payload, and uses generic payload visitation on the resulting protobuf object to apply the operation to all contained payloads instead. Then it reserializes the envelope.

### Context

Each system nexus operation could potentially need a different serialization context from each other, and often not that used for a normal nexus operation. For example, signal with start's internal user payloads must have the target workflow serialization context, so that they can be appropriately deserialized/decoded by the target workflow. For this reason, serialization context for system nexus operations must be derived from the operations themselves through generated code. Currently this is a function annotation which is given the operation input and must produce a serialization context.

### Responses

Responses to the system nexus operation will also be marked as system payloads. They will go through the same process and should use the same context as that used for its outgoing request.

## Interception

Each system nexus operation needs two points of interception which serve different purposes. First, each operation has its own interception point specific to that operation, which receives the whole input request type. This gives access to the arguments/headers/etc which are received by the target, and should be used to implement tracing headers and most other interception behavior.

The second interception point is a generic one for all system nexus operations. This receives the actual outer nexus operation's input rather than the envelope. It is functionally equivalent to the existing nexus interception point, but distinguished because many interceptors may choose to do different things (tracing ones should usually log in the specific one rather than the generic one for instance). This is primarily important for a scenario such as needing to apply headers for an auth proxy. Any headers attached here would otherwise be rejected by the server today.

## Adding an operation

Adding a System Nexus operation requires generated transfer types, a
serialization-context factory, a discoverable protobuf service and method
descriptor, and payload-visitor coverage for every nested payload field.

## Language Specific Considerations

### TypeScript

In Typescript, it is difficult to actually create the proto binary envelope in the isolate due to read only considerations with the protobufjs library. Instead, the isolate serializes the envelope with the normal json converter, and then processing of that payload during codec/external storage application converts it to proto binary. Notably this happens and needs to happen regardless of the presence of a codec or external storage.

Additionally, the serialization context is not retrievable outside the isolate because the factory is defined as taking the user model type rather than the transfer type, which is what remains after leaving the isolate. For this reason, the json serialized envelope has an additional `__temporal_system_context` metadata which contains the serialization context so it can be reused outside the isolate.
