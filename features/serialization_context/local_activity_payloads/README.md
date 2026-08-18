# Serialization context: local activity payloads

A local activity gets an `ActivitySerializationContext` with `IsLocal = true`,
while the marker bookkeeping around it stays workflow scoped.

Steps:

- register a payload codec that stamps the signature of its serialization
  context onto every payload it encodes, and rejects payloads that were encoded
  under a different context
- run a local activity
- verify the client result
- verify that the `LocalActivity` marker `data` payload carries the workflow
  signature
- verify that the `LocalActivity` marker `result` payload carries the activity
  signature with `IsLocal = true`

Not implemented for TypeScript: the SDK has no local activities.

## Known Go SDK gap

This feature currently fails on the Go SDK. `WithLocalActivityTask` builds the
local activity environment from the worker's plain data converter instead of
`ExecuteLocalActivityParams.DataConverter`, so the result is encoded without any
context, while `ExecuteLocalActivity` leaves the future on the workflow context,
so the same payload is decoded as workflow scoped. Any context aware converter
therefore breaks local activities.
