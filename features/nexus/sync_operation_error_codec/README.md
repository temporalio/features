# Synchronous operation error codec

A synchronous Nexus operation fails with an `OperationError` wrapping an
`ApplicationFailure` (`ApplicationError` in Go). Common failure attribute encoding
is enabled, so both the wrapper and its application cause store their messages and
stack traces in codec-encoded `encoded_attributes` payloads. Application failure
details also pass through the codec.

The workflow checks the decoded wrapper, original messages, application cause, and
detail. The history assertions verify encoded common attributes on both failures,
placeholder messages and empty raw stack traces, and the encoded application detail.
