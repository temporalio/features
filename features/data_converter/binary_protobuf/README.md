# Binary protobuf payload converter

Protobuf values can be converted to and from `binary/protobuf` Payloads.

This feature:

- runs a [`BinaryMessage`](../messages.proto) with a single byte `0000 0101` for `data` through the Binary Protobuf
  Payload Converter, writes it to `payloads/binary_protobuf.[lang]`, and verifies it matches the other files in
  `payloads/`
- decodes all files in `payloads/` with the default Payload Converter and verifies the value is a `Binary Message` with
  `data: 0000 0101`
