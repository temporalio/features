# Synchronous result codec

A synchronous Nexus operation returns a result through a payload codec. The workflow
checks the decoded result, and the feature checks that the raw input and result in
history use the codec's encoding.
