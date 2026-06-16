# Worker shutdown: poll completion on shutdown

This feature verifies worker shutdown behavior with
`frontend.enableCancelWorkerPollsOnShutdown` enabled and disabled.

The feature has two run variants in `config.json`:

- `cancel-worker-polls-enabled` starts the embedded dev server with
  `frontend.enableCancelWorkerPollsOnShutdown=true`.
- `cancel-worker-polls-disabled` starts the embedded dev server with
  `frontend.enableCancelWorkerPollsOnShutdown=false`.

Each language implementation starts several workflows that repeatedly run a
short timer and no-op activity, waits until activity work is scheduled, stops
the worker, and verifies shutdown returns promptly. The test uses
runner-provided capability metadata for variant-specific history assertions:
the enabled variant verifies workflow histories do not contain workflow-task
failures or timeouts, and the disabled variant verifies at least one workflow
history does contain a workflow-task failure or timeout.

Ruby is intentionally not implemented for this feature yet. The Ruby feature
harness does not expose worker shutdown control to the feature, so a Ruby test
cannot currently stop the worker during active polling or run the same
mode-specific assertions as the other SDKs.

Run both variants for a language with:

```bash
go run . run --lang [go|java|ts|py|cs] --no-history-check worker_shutdown/poll_complete_on_shutdown
```
