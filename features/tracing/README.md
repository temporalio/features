# Tracing

SDKs support the tracing of various operations, and will automatically trace activity
and workflow lifecycles if enabled.

They also should expose, via context, interceptors or similar mechanisms, methods for
users to trace their own operations

# Detailed Spec

The tracing API exposed to users should support exporting traces to multiple common
backends. OpenTelemetry APIs, where feasible, can be used as the main interface for
users.