# JSON payload converter

JSON values can be converted to and from `json/plain` Payloads.

This feature: 

- runs the JSON value `{ "spec": true }` through the default Payload Converter, writes it to `payloads/json.[lang]`, and
  verifies it matches the other files in `payloads/`
- decodes all files in `payloads/` with the default Payload Converter and verifies the JSON value is `{ "spec": true }`

# Detailed spec

`metadata.encoding = toBinary("json/plain")`

- If JSON encoding fails, JsonPayloadConverter returns null/undefined. Since it's the last converter in
  CompositePayloadConverter, CompositePayloadConverter.toPayload throws.