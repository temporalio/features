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

Not implemented for Go: the SDK built the local activity environment from the
worker's plain data converter, so the result was encoded without any context and
then decoded as workflow scoped. temporalio/sdk-go#2562 fixes this. Add
`feature.go` back, together with its entry in `features/features.go`, once that
fix is in a release.
