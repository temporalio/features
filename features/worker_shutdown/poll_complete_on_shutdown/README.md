# Worker shutdown: poll completion on shutdown

This feature verifies worker shutdown behavior with
`frontend.enableCancelWorkerPollsOnShutdown` enabled and disabled.

The feature has two run variants in `.config.json`:

- `cancel-worker-polls-enabled` starts the embedded dev server with
  `frontend.enableCancelWorkerPollsOnShutdown=true`.
- `cancel-worker-polls-disabled` starts the embedded dev server with
  `frontend.enableCancelWorkerPollsOnShutdown=false`.

Each language implementation starts several workflows that repeatedly run a
short timer and no-op activity, waits until activity work is scheduled, stops
the worker, and verifies shutdown returns promptly. In the enabled variant, the
test uses runner-provided capability metadata to also verify workflow histories
do not contain workflow-task failures or timeouts.

TODO: Ruby currently has lighter coverage because the Ruby feature harness does
not expose worker shutdown control to the feature. Extend the Ruby harness so
this feature can stop the worker during active polling and run the same
mode-specific assertions as the other SDKs.

Run both variants for a language with:

```bash
go run . run --lang [language] --no-history-check worker_shutdown/poll_complete_on_shutdown
```
