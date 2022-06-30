# Compression codec

Payloads can be run through a `binary/zlib` compression Payload Codec.

This feature:

- runs the JSON value `{ "spec": true }` through the default Payload Converter, runs the Payload through the zlib
  compression Codec, writes it to `payloads/compressed.[lang]`, and verifies it matches the other files in `payloads/`
- decodes all files in `payloads/` with the compression Codec and default Payload Converter and verifies the JSON value
  is `{ "spec": true }`

# Detailed spec

`metadata.encoding = toBinary("binary/zlib")`
`data = [json payload]`