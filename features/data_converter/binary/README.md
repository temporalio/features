# Binary payload converter

Binary values can be converted to and from `binary/plain` Payloads.

This feature: 

- runs the binary value `101` (5) through the default Payload Converter, writes it to `payloads/binary.[lang]`, and
  verifies it matches the other files in `payloads/`
- decodes all files in `payloads/` with the default Payload Converter and verifies the binary value is `101`

# Detailed spec

`metadata.encoding = toBinary("binary/plain")`