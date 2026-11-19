# Nexus try cancellation does not wait for completion

A workflow starts one workflow-backed Nexus operation, waits for its token, and cancels
its scope using `TRY_CANCEL`. It handles local cancellation and completes while the
backing workflow is still waiting for a release signal.

After the caller completes, the harness releases the backing workflow and checks its
result. Cancellation delivery is not guaranteed once the caller exits, so the backing
workflow may either be canceled or complete successfully.

The caller's history contains scheduled, started, and cancellation-requested Nexus
events. Both workflow histories are replayed.
