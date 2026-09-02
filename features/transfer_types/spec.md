# Transfer Type Converter Behavioral Specification

Status: Draft

Last updated: 2026-09-02

## Purpose

This document defines the portable behavior of Temporal transfer type converters. It specifies observable conversion semantics without prescribing SDK-specific APIs, generic type mechanisms, or converter construction strategies.

## Terminology

- **Model type:** The user-facing type associated with a transfer type converter.
- **Transfer type:** The serializer-facing type produced from a model value.
- **Transfer value:** A value of the transfer type.
- **Payload conversion:** Conversion between an in-memory value and a Temporal `Payload`.
- **Payload codec:** A byte-to-byte transformation applied to a payload, such as encryption or compression.
- **Failure details:** Typed payloads stored in application failures, cancellations, timeout heartbeat details, reset failures, or nested failure causes.

## Normative behavior

### Conversion order

For outbound values, an SDK MUST apply operations in this order:

1. Locate the transfer converter from the top-level model value's runtime type.
2. Invoke the transfer converter when one is present.
3. Pass the resulting transfer value to the configured payload converter.
4. Apply external storage processing and payload codecs according to the SDK's existing data-converter contract.

For inbound values, an SDK MUST apply the inverse order:

1. Apply payload codecs and external storage retrieval according to the SDK's existing data-converter contract.
2. Locate the transfer converter from the requested model type.
3. Ask the configured payload converter to decode the payload as the transfer type.
4. Invoke the transfer converter to reconstruct the requested model value.

The transfer layer MUST NOT replace or bypass the configured payload converter, failure converter, payload codecs, external storage provider, or serialization context.

### Top-level values only

Transfer conversion MUST apply only to values directly handed to the SDK payload-conversion boundary. It MUST NOT recursively inspect fields, properties, collection elements, or other nested values.

Nested serialization remains the responsibility of the configured payload converter.

### Exactly one transfer step

An SDK MUST perform at most one transfer conversion for a top-level value.

If model type `A` converts to transfer value `B`, and `B` also has a transfer converter, the SDK MUST pass `B` directly to the configured payload converter. It MUST NOT invoke the converter associated with `B`.

On decode, the payload converter MUST decode directly to the selected transfer type. Only the converter associated with the requested model type may reconstruct the final value.

### Encode and decode type selection

Encoding MUST select a converter using the top-level value's runtime type. A null outbound value has no runtime model type and MUST pass directly to the configured payload converter.

Decoding MUST select a converter using the requested user-facing type. If the SDK has no requested type information, it MUST return the configured payload converter's normal result without applying transfer reconstruction.

The SDK MUST provide the requested model type information available in its type system when selecting the transfer type and reconstructing the model value. This includes generic arguments when the SDK's conversion API preserves them.

### Converter declaration and lookup

A transfer converter declaration applies only to the exact model type on which it is declared. Declarations MUST NOT be inherited from a base type.

A subclass or other derived type MAY declare its own converter independently of its base type.

Runtime encoding MUST look only for a converter declared by the exact runtime type. Inbound decoding MUST look only for a converter declared by the exact requested type.

Every selected converter MUST provide a concrete, non-null transfer type for inbound payload conversion. The SDK MUST fail clearly if the converter does not provide a valid transfer type. It MUST NOT fall back to payload metadata inference after selecting a converter.

### Transfer value nullability

When a converter has been selected for an inbound model type, the SDK MUST invoke its reconstruction hook even when the decoded transfer value is null.

This permits a non-null model value to use null as its transfer representation.

An outbound null model value MUST pass through without converter lookup because no runtime model type is available.

### Failure conversion

Failure semantics MUST remain owned by the configured failure converter.

The failure converter MUST use the same transfer-aware payload-conversion path used for ordinary arguments and results when encoding or decoding typed failure payloads. This includes:

- Application failure details.
- Cancellation details.
- Timeout last-heartbeat details.
- Reset workflow failure details.
- Details contained in nested failure causes.

Payload codecs and external storage transformations MUST be applied exactly once to failure payloads and in their normal order.

Transfer conversion does not apply to untyped failure metadata such as messages, stack traces, failure types, retry state, or encoded failure attributes. Existing payload-codec behavior for encoded failure attributes remains unchanged.

### Raw payloads

A raw payload wrapper MUST preserve its existing pass-through behavior. A transfer converter MUST NOT reinterpret a raw payload wrapper as a model value.

### Configured converter authority

The configured payload converter remains authoritative over the transfer representation's wire format. Transfer conversion MUST NOT add transfer-specific payload metadata or choose a wire encoding independently.

The configured failure converter remains authoritative over the mapping between language exceptions and Temporal failure protos.

### Serialization context

Transfer conversion MUST preserve serialization context propagation to payload converters, failure converters, codecs, and external storage components.

### Converter lifetime and concurrency

Converter construction, lookup, and invocation MUST be safe under concurrent serialization. An SDK MAY reuse converter instances, so converter implementations MUST satisfy the concurrency requirements documented by that SDK.

The number of converter instances and the time at which they are constructed are not part of the portable behavior.

### Invalid declarations and callback failures

An invalid converter declaration MUST fail clearly before conversion completes. Examples include an incompatible converter, a converter that cannot be constructed according to the SDK's API, or an invalid transfer type.

Exceptions raised by transfer callbacks MUST remain observable as conversion failures. The SDK MUST NOT silently fall back to ordinary model serialization after selecting a converter.

## Conformance requirements

An implementation conforms to this specification when all of the following hold:

- Ordinary arguments and results satisfy the normative conversion order.
- Failure details use transfer conversion without bypassing a configured failure converter.
- Payload codecs and external storage transformations are applied exactly once.
- A selected converter reconstructs from a null transfer value.
- Top-level-only and one-step behavior have explicit regression tests.
- Raw payload pass-through remains intact.
- Context propagation remains intact.
- Exact converter lookup is implemented and documented as non-inherited behavior.
- Workflow history containing transfer-represented values can be replayed.

## Required conformance tests

### Payload conversion

- Annotated argument and result round trip.
- Mixed annotated and ordinary values preserve position and type.
- Missing payloads still produce language-default values.
- Available requested type information reaches converter selection and reconstruction.
- Null outbound model passes through.
- Non-null model represented by null reconstructs through its hook.
- Raw payload wrapper passes through unchanged.

### Conversion boundaries

- Annotated `A` converts to annotated `B` without invoking `B`'s converter.
- Annotated nested values do not invoke their converters.
- An unannotated subclass does not inherit an annotated base converter.
- A subclass can declare its own converter independently.

### Failures

- Application failure details round trip.
- Cancellation details round trip.
- Timeout last-heartbeat details round trip.
- Nested failure causes round trip their details.
- Custom failure converter remains authoritative.
- Payload codecs transform each failure payload exactly once.

### Integration surfaces

- Workflow client and worker.
- Workflow history replay.
- Activity stubs and standalone activity client.
- Local activities and activity heartbeats.
- Nexus client and worker.
- Schedule client create and describe operations.
- Test activity environment.
