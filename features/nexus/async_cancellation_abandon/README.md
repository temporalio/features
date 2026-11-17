# Nexus cancellation abandons the operation

A workflow starts one workflow-backed Nexus operation, waits for its token, and cancels
its scope using `ABANDON`. It handles local cancellation without requesting cancellation
of the operation.

After the caller completes, the harness sends the backing workflow a release signal
and checks that it completes successfully rather than being canceled.

The caller's history contains only scheduled and started Nexus events. Both workflow
histories are replayed.
