# Encryption codec

Payloads can be run through a `binary/encrypted` encryption Payload Codec that uses AES GCM with a 256-bit key.

This feature:

- runs the JSON value `{ "spec": true }` through the default Payload Converter, runs the Payload through the zlib
  encryption Codec, writes it to `payloads/encrypted.[lang]`, and verifies it matches the other files in `payloads/`
- decodes all files in `payloads/` with the encryption Codec and default Payload Converter and verifies the JSON value
  is `{ "spec": true }`

# Detailed spec

`metadata.encoding = toBinary("binary/encrypted")`
`data = [json payload]`