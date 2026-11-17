# Nexus cancellation waits for delivery

A workflow starts one workflow-backed Nexus operation, waits for its token, and cancels
its scope using `WAIT_REQUESTED`. It handles local cancellation and completes without
waiting for the backing workflow to finish.

The backing workflow defers completion until the harness sends a release signal after
the caller completes. It must have received cancellation and finish as canceled.

The caller's history contains scheduled, started, and cancellation-requested Nexus
events. Both workflow histories are replayed.
