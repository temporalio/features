# Encoded tracing headers across languages

Reproduces [.NET → TypeScript encoded tracing headers](https://github.com/temporalio/sdk-typescript/issues/514#issuecomment-6259375934).
A .NET client using its tracing interceptor and payload codec starts a workflow
on a TypeScript OpenTelemetry worker using the matching codec.

The driver currently runs only .NET → TypeScript. It requires the workflow to
complete without Workflow Task failures, round-trip its input/result, and retain
the caller's trace identity. It verifies that the history's `_tracer-data` header
was actually codec-encoded. Silently discarding an unreadable trace header is not
a pass.

Both codecs wrap serialized protobuf `Payload` bytes with XOR `0x5a`, setting
`encoding` to `binary/encrypted`. This is a test codec, **not encryption**.
Both reject double encoding.

From the repository root, with the usual Go, .NET, Node and Corepack/pnpm prerequisites:

```sh
go run . run-cross-language --sdk-version cs=1.21.1 --sdk-version ts=1.25.0 tracing/encoded_headers
```

This is a **failing regression test** with those releases: TypeScript raises
`Unknown encoding: binary/encrypted` in `extractContextFromHeaders`. It is not
marked as an expected failure. The workflow has a 15-second execution timeout;
the driver has a one-minute scenario deadline and prints the original task failure.

To select SDK changes under development, replace either version with a checkout
path (relative paths are resolved from the repository root):

```sh
go run . run-cross-language --sdk-version cs=1.21.1 --sdk-version ts=/path/to/sdk-typescript tracing/encoded_headers
```

The TS feature requests header codec processing with `encodeHeaders: true`.
SDKs without that option ignore it and fail the test. There is no test interceptor
that decodes the header on the SDK's behalf.

The runner reuses the normal SDK preparation and dev-server setup. The driver
launches the existing TS harness in worker-only mode and the .NET harness in
client-only mode. It stops the worker on success or failure. No external tracing
backend is used, and no C# source is embedded in the driver.

Further languages and scenarios belong in this feature's driver and participants,
not in a Cartesian-product policy in the runner. Single-language history replay
is not applicable here; the .NET participant checks the actual TS execution history.
