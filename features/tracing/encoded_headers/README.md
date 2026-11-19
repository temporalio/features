# Encoded tracing headers across languages

Verifies that a client from one SDK can start a workflow on another SDK using
codec-encoded tracing headers while preserving trace context. Every client checks that:

- History contains a codec-encoded `_tracer-data` header with W3C `traceparent`
  retaining the caller's trace identity.
- The workflow completes and round-trips its input and result through the codec.
- The workflow tracing span retains the caller's trace identity.
- No Workflow Task failed, even if a subsequent attempt succeeded.

Each participant uses its SDK's tracing integration and a test codec that wraps
the serialized protobuf `Payload` with XOR `0x5a` and sets
`metadata.encoding=binary/encrypted`. This deterministic codec is **not encryption**.
Its decoder requires encoded payloads, and its encoder rejects double encoding.

## Participants

All participants support both client and worker roles.

| SDK (language code) | Tracing configuration |
| --- | --- |
| .NET (`cs`) | `TracingInterceptor` and client codec |
| Go (`go`) | OpenTelemetry interceptor with `TracerOptions.EncodeHeaders: true` and client codec |
| Java (`java`) | OpenTracing interceptors with the OpenTelemetry shim, W3C propagator, `setEncodeHeaders(true)`, and client codec |
| Python (`py`) | `TracingInterceptor`, `HeaderCodecBehavior.CODEC`, and client codec |
| Ruby (`rb`) | OpenTelemetry tracing interceptor, client codec, and worker codec thread pool |
| Rust (`rs`) | OpenTelemetry plugin with `encode_client_headers(true)` and worker activation codecs |
| TypeScript (`ts`) | OpenTelemetry plugin with `encodeClientHeaders: true`, worker `codecHeaders: true`, and payload codecs |

## Running

The driver runs every client/worker pair among the prepared SDKs, including
same-language pairs. Each pair uses its own task queue and a 45-second timeout.
Failures identify the client and worker SDKs. No tracing backend is required.

Run all 49 pairs with these tested SDK versions:

```sh
go run . run-cross-language \
  --sdk-version cs=1.22.0 --sdk-version go=1.51.0 \
  --sdk-version java=1.41.1 --sdk-version py=1.36.1 \
  --sdk-version rb=1.11.0 --sdk-version rs=1.2.0 \
  --sdk-version ts=1.26.0 tracing/encoded_headers
```

To run selected pairs, set `ENCODED_HEADERS_PAIRS` to a comma-separated list of
`client:worker` language codes. For example, to run a .NET client with a TypeScript
worker:

```sh
ENCODED_HEADERS_PAIRS=cs:ts go run . run-cross-language \
  --sdk-language cs --sdk-language ts --sdk-version cs=1.22.0 \
  --sdk-version ts=1.26.0 tracing/encoded_headers
```
