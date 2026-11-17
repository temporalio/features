# Retry synchronous result encoding

A payload codec fails once while encoding a synchronous Nexus result. The operation
must retry and return the decoded result to the workflow. Handler and response encoder
counts must both be two, and history must contain just one scheduled operation and
one encoded completion, with no asynchronous start.
