# Signal-with-start from a workflow: ALLOW_DUPLICATE_FAILED_ONLY reuse policy

Covers both sub-cases of `id_reuse_policy=ALLOW_DUPLICATE_FAILED_ONLY` when called
from a workflow via `workflow.signal_with_start_workflow(...)`:

1. Target completed successfully -> the operation is rejected.
2. Target was terminated (a failed/closed outcome) -> a new run is started.

Verifies: case 1 fails; case 2 starts a new run with a different run id.

## Server requirements
Namespace dynamic config `history.enableSignalWithStartFromWorkflow=true`,
`history.enableChasm=true`, and the built-in `__temporal_system` Nexus
endpoint. Also set
`system.workflowIdReuseMinimalInterval=0` to avoid reuse-interval throttling.
