# Encoded tracing headers across languages

Exercises real SDK tracing integrations across workflow starts, including the
original [.NET → TypeScript regression](https://github.com/temporalio/sdk-typescript/issues/514#issuecomment-6259375934).
No interceptor decodes headers on an SDK's behalf. Every client checks that:

- History contains a codec-encoded `_tracer-data` header with W3C `traceparent`
  retaining the caller's trace identity.
- The workflow completes and round-trips its input and result through the codec.
- The real workflow tracing span retains that identity (silently dropping an
  unreadable header is not a pass).
- No Workflow Task failed, even if a subsequent attempt succeeded.

All participants wrap the entire serialized protobuf `Payload` with XOR `0x5a`
and set `metadata.encoding=binary/encrypted`. This deterministic test codec is
**not encryption**. Every codec rejects double encoding. Raw client-supplied
Payloads are wire-ready; this feature does not install a converter-wide wrapper
or manually rewrite tracing headers.

## Participants

| SDK | Encoded tracing client | Worker | Real integration/configuration |
| --- | --- | --- | --- |
| .NET (`cs`) | Yes | Yes | `TracingInterceptor`, configured client codec |
| Go (`go`) | Yes | Yes | OpenTelemetry interceptor with producer-specific `TracerOptions.EncodeHeaders: true`; client codec |
| Java (`java`) | Yes | Yes | OpenTracing interceptors with the OpenTelemetry shim/W3C propagator and producer-specific `setEncodeHeaders(true)`; client codec |
| Python (`py`) | Yes | Yes | `TracingInterceptor`, client `HeaderCodecBehavior.CODEC` and codec |
| Ruby (`rb`) | Yes | Yes | OpenTelemetry tracing interceptor and client codec; worker codec thread pool |
| Rust (`rs`) | Yes | Yes | OpenTelemetry plugin with `encode_client_headers(true)` and existing worker activation codec traversal |
| TypeScript (`ts`) | Yes | Yes | OpenTelemetry plugin with `encodeClientHeaders: true`, worker `codecHeaders: true`, and payload codecs |
| PHP (`php`) | No | No | No SDK tracing integration in the inspected checkout; no participant is fabricated |

Rust and TypeScript encode only the tracing carrier produced by their plugins;
other client-supplied Payload headers remain wire-ready and untouched. These
producer opt-ins are independent of worker processing: Rust workers already decode
activation headers, while TypeScript workers use `codecHeaders`. The Rust
converter-wide encoding proposal is not needed or reinstated.
PHP would first need a real tracing integration and a codec/header policy.

## Running

By default, the runner discovers and prepares all participant files once. The
feature driver runs every supported client/worker pair among the prepared SDKs,
including same-language controls. Each pair gets its own queue and a 45-second
deadline; failures are labeled and aggregated while later pairs continue. Workers
are stopped and reaped on success, failure and cancellation. No tracing backend
is required.

The original .NET → TypeScript case remains independently runnable:

```sh
go run . run-cross-language --sdk-language cs --sdk-language ts --sdk-version cs=1.21.1 --sdk-version ts=/path/to/sdk-typescript tracing/encoded_headers
```

With `ts=1.25.0` this is a **failing regression test**, not an expected-failure
success: tracing extraction raises `Unknown encoding: binary/encrypted`.
The fixed TypeScript worker uses `codecHeaders`, not the removed data-converter
`encodeHeaders` flag.

To validate the Go and Java producer opt-ins against branches under development:

```sh
go run . run-cross-language --sdk-language go --sdk-language java --sdk-language py --sdk-version go=/path/to/sdk-go --sdk-version java=/path/to/sdk-java tracing/encoded_headers
```

Local Go selection also substitutes its independently versioned OpenTelemetry
module; local Java selection substitutes its OpenTracing module. Older releases
without the opt-ins cannot run these producer participants and fail rather than
silently testing unencoded headers. SDKs must be fixed/merged before this feature
change lands.

Rust and TypeScript producer cases likewise require SDKs containing their new
OTel plugin flags. Select those checkouts with `--sdk-version rs=/path/to/sdk-rust`
and `--sdk-version ts=/path/to/sdk-typescript` while the changes are unreleased.

Use `ENCODED_HEADERS_PAIRS` only when focusing on particular scenarios:

```sh
ENCODED_HEADERS_PAIRS=py:rs go run . run-cross-language --sdk-language py --sdk-language rs --sdk-version rs=/path/to/sdk-rust tracing/encoded_headers
```

The filter is a comma-separated list of `client:worker` language codes. Requested
unsupported roles or unprepared participants are errors; they never disappear
from the run or count as passes. `--sdk-language` only limits preparation; all
pair/scenario policy stays in this feature's driver. Ruby preparation requires
the Ruby version specified by the harness gemspec; Rust requires the package's
pinned compiler/protoc and builds its `cross-language` Cargo feature.

Single-language history comparison/replay is intentionally not used: the client
participant checks the actual other-language workflow history.
