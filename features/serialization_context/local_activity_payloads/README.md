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
