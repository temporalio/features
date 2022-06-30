# Null payload converter

Null or undefined values can be converted to and from `binary/null` Payloads.

This feature: 

- runs a null/undefined value through the default Payload Converter, saves it to `payloads/null.[lang]`, and verifies it matches
  the other files in `payloads/`
- decodes all files in `payloads/` with the default Payload Converter and verifies the null/undefined value is returned