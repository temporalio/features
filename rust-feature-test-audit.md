# Rust feature test audit

Audit date: 2026-08-26

Scope: all 54 Rust feature implementations added on `olszewski/feat_rust_harness`, their shared harness, CI configuration, and the corresponding feature READMEs.

Use the checkboxes below to track resolution. A finding should be checked only after its stated acceptance criteria pass.

When resolving findings, heavily avoid overcomplicating feature tests. Where possible, use an existing implementation in another language as the model and preserve its structure and intent. Add Rust-specific orchestration or helpers only when the simpler cross-language port cannot satisfy the acceptance criteria.

## Findings

### Critical

- [x] **CI builds the configured Rust SDK version.**

  CI now requests Rust SDK `0.7.0` in [`.github/workflows/ci.yaml`](.github/workflows/ci.yaml#L342), matching the `0.7` harness dependencies. The generated feature program resolves one compatible SDK dependency graph.

  Reproduction:

  ```bash
  go run . run --lang rs --version 0.7.0 --no-history-check signal/basic
  ```

  Acceptance criteria:

  - The SDK version exercised by CI and the harness dependency version are compatible. ✓
  - The reproduction command builds and runs the feature successfully. ✓
  - CI exercises the current supported Rust SDK version. ✓

  Resolution: pending commit; verified with `go run . run --lang rs --version 0.7.0 --no-history-check signal/basic`. CI intentionally tests the suite's current supported Rust SDK `0.7.0`, rather than the previous incompatible `0.6.0` target.

### High

- [x] **`signal/prevent_close` is racy and fails live.**

  [`features/signal/prevent_close/feature.rs`](features/signal/prevent_close/feature.rs#L31) creates a completion window with a 300 ms wall-clock sleep. The workflow can complete before the second signal RPC reaches the server. Both a targeted run and the full suite failed with `workflow execution already completed`.

  Acceptance criteria:

  - The test uses deterministic synchronization that proves the second signal is pending while workflow completion is attempted.
  - The test reliably observes both signals and the replay/redispatch behavior described in the README.
  - Repeated targeted runs pass without timing-dependent sleeps.

- [x] **`update/validation_replay` never causes a replay.**

  [`features/update/validation_replay/feature.rs`](features/update/validation_replay/feature.rs#L87) executes an update and completes the normally cached workflow, then concludes that validation was skipped because the validator counter remains one. It neither evicts the workflow cache nor invokes a Rust history replayer, so it can pass without exercising replay.

  Acceptance criteria:

  - The test demonstrably forces a replay after the update is accepted. ✓
  - The validator would fail if invoked during that replay. ✓
  - The replay completes successfully and the validator invocation count proves it was skipped. ✓

  Resolution: the Rust feature disables workflow caching, forcing each workflow task after the update is accepted to replay the full history. The workflow completes while the validator counter remains one. Verified with `go run . run --lang rs --version 0.7.0 --no-history-check update/validation_replay`.

- [x] **The eager-activity test omits the worker configuration it claims to test.**

  The README requires activities to be registered while non-local activity polling is disabled. [`features/eager_activity/non_remote_activities_worker/feature.rs`](features/eager_activity/non_remote_activities_worker/feature.rs#L55) registers only the workflow. Its schedule-to-start timeout therefore proves only that no activity handler exists; it cannot detect incorrect eager execution when the activity is registered on a workflow-only worker.

  Acceptance criteria:

  - The activity is registered on the worker.
  - Remote activity polling is explicitly disabled while workflow polling remains active.
  - The eager activity does not execute, the workflow observes a schedule-to-start timeout, and the worker remains healthy.

  Resolution: Implemented; manual review pending. Fix commit: `03fb30c557557c66b466b9c8d3f3e5d3c43b6976`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check eager_activity/non_remote_activities_worker`.

- [x] **`data_converter/failure` tests the wrong failure boundary and history event.**

  The README requires an activity that fails with a nested cause and inspection of the `ActivityTaskFailed` event. [`features/data_converter/failure/feature.rs`](features/data_converter/failure/feature.rs#L109) fails directly from the workflow and inspects `WorkflowExecutionFailed`, bypassing activity failure conversion.

  Acceptance criteria:

  - The workflow invokes an activity.
  - The activity returns a failure with a nested cause.
  - The test inspects `ActivityTaskFailed` and verifies encoded attributes on both the outer failure and its cause.

  Resolution: Implemented; manual review pending. Fix commit: `44883e3327bb105db883b9fef3b157f7c76b57c0`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check data_converter/failure`.

- [x] **The continue-as-new update test resubmits the update after the behavior under test fails.**

  In [`features/continue_as_new/updates_do_not_block_continue_as_new/feature.rs`](features/continue_as_new/updates_do_not_block_continue_as_new/feature.rs#L150), a `ResourceExhausted` error from the update pinned to the original run triggers a manual `start_update` call against the current execution. This lets the test pass even if the SDK fails to carry or retry the originally admitted update across continue-as-new.

  Acceptance criteria:

  - A single update request is admitted while the pre-continue workflow task is in flight.
  - No test-side resubmission occurs after continue-as-new.
  - The original update handle completes on the post-continue run.
  - Update accepted/completed events exist only in the post-continue history.

  Resolution: Implemented; manual review pending. Fix commit: `4f7c0478dd58f036c2cb847232428b606f232672`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check continue_as_new/updates_do_not_block_continue_as_new`.

### Medium

- [x] **Several data-converter tests use self-fulfilling serialization checks.**

  - [`data_converter/binary`](features/data_converter/binary/feature.rs#L22) uses the same `binary_payload` helper for serialization and its expected value instead of loading `payload.json` as specified.
  - [`data_converter/empty`](features/data_converter/empty/feature.rs#L24) similarly creates `binary/null` with the same helper on both sides instead of comparing with its checked-in fixture.
  - [`data_converter/json_protobuf`](features/data_converter/json_protobuf/feature.rs#L27) implements its own JSON/protobuf serializer and deserializer, so it verifies test code rather than SDK-provided protobuf conversion.

  Acceptance criteria:

  - Binary and null payload tests compare history payloads against their independent checked-in fixtures.
  - JSON/protobuf exercises the SDK converter intended by the feature, or the README explicitly documents why a custom converter is the feature being tested.
  - Expected values are not produced by the same helper responsible for the behavior under test.

  Resolution: Implemented; manual review pending. Fix commit: `56d9b2a74228f25339388715f17d1ef833103302`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check data_converter/binary data_converter/empty data_converter/json_protobuf`. Deliberate README deviation: `data_converter/json_protobuf/README.md` documents that Rust SDK 0.7 has no SDK-provided `json/protobuf` converter and validates its minimal converter against an independent fixture.

- [x] **The inactive-worker query test can pass without a deadline timeout.**

  [`features/query/timeout_due_to_no_active_workers/feature.rs`](features/query/timeout_due_to_no_active_workers/feature.rs#L48) starts the workflow on a queue that has never had a worker and accepts `FailedPrecondition` in addition to `Cancelled` or `DeadlineExceeded`. An immediate precondition failure therefore satisfies a README that specifically requires a timeout according to the client deadline.

  Acceptance criteria:

  - The workflow first processes a task and successfully answers a query.
  - Its worker is stopped before the tested query.
  - The tested query waits until the configured deadline and fails only with the SDK's documented cancellation/deadline status.
  - The worker is restarted and the workflow is completed cleanly.

  Resolution: Implemented; manual review pending. Fix commit: `a053937802206db21bdfe2fd4a6c310d7e16fad6`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check query/timeout_due_to_no_active_workers`.

- [x] **Schedule backfill accepts four executions although the README requires six.**

  [`features/schedule/backfill/feature.rs`](features/schedule/backfill/feature.rs#L80) accepts an action count of either four or six. The README explicitly says both ranges are inclusive and requires six executions.

  Acceptance criteria:

  - The test requires exactly six completed backfill actions.
  - If older server behavior must remain supported, that exception is version-gated and documented in the README instead of weakening the assertion unconditionally.

  Resolution: Implemented; manual review pending. Fix commit: `a5768339f531c4302b4e78045925acb65795417c`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check schedule/backfill`.

- [ ] **`update/async_accepted` does not reconstruct a handle from identifying information.**

  [`features/update/async_accepted/feature.rs`](features/update/async_accepted/feature.rs#L82) calls `start_update` a second time with the same update ID and relies on deduplication. It does not obtain a handle using only the workflow ID, run ID, and update ID as described by the README.

  Acceptance criteria:

  - After acceptance, the original caller can be dropped.
  - A new handle is created using the identifying information documented in the README, without resubmitting the update handler invocation and arguments.
  - The reconstructed handle returns the same successful or failed outcome as the original call.

- [x] **Worker shutdown timeouts are converted into successful feature results.**

  [`harness/rust/src/lib.rs`](harness/rust/src/lib.rs#L347) logs a worker that fails to stop within ten seconds and then returns `Ok(())`. This hides worker lifecycle regressions across otherwise passing features.

  Acceptance criteria:

  - A worker shutdown timeout fails the owning feature with a descriptive error.
  - Any SDK-specific exception is narrowly scoped, documented, and does not turn unrelated shutdown failures into passes.

  Resolution: Implemented; manual review pending. Fix commit: `2c968def200602d7cb3268214dd5d5c44aec0506`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check signal/basic`.

- [x] **`child_workflow/cancel_abandon` does not test cancellation of child start.**

  The README says to cancel the start context, verify that start throws immediately, and verify that the child receives no cancellation request. [`features/child_workflow/cancel_abandon/feature.rs`](features/child_workflow/cancel_abandon/feature.rs#L58) waits for child start to succeed, cancels its result/parent later, and returns a synthetic `"cancelled"` value.

  Acceptance criteria:

  - The cancellation targets the child-start operation described in the README.
  - The parent immediately observes the expected cancellation error.
  - The child remains running and can complete normally after being signaled.

  Resolution: Implemented; manual review pending. Fix commit: `c6341662ed23bc022de1c9dd1c493da2d7f6a9a1`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check child_workflow/cancel_abandon`.

- [x] **`update/non_durable_reject` does not verify rejection-only workflow-task durability.**

  The detailed README requires that scheduled/started/completed workflow-task events do not land in history when a workflow task contains only rejected updates. [`features/update/non_durable_reject/feature.rs`](features/update/non_durable_reject/feature.rs#L112) checks only that no `WorkflowExecutionUpdateRejected` event exists.

  Acceptance criteria:

  - At least one workflow task is known to contain only rejected updates.
  - History event counts or IDs prove that task's scheduled/started/completed events were not persisted.
  - Accepted updates still mutate state and appear normally in history.

  Resolution: Implemented; manual review pending. Fix commit: `4008d1fdc57aa0169ab6e56ae11815f371123212`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check update/non_durable_reject`.

### Coverage and documentation

- [ ] **No Rust history fixtures are checked in.**

  There are zero `history.rs.<version>.json` files for the 54 Rust features. This conflicts with the history guidance in [`README.md`](README.md#L231) and causes Rust cross-version history comparison to be skipped.

  Acceptance criteria:

  - Each eligible Rust feature has history generated at its earliest supported SDK version.
  - CI runs without `--no-history-check` and performs real Rust history comparisons.
  - Features intentionally exempt from history have a documented, machine-readable exemption.

- [x] **`schedule/duplicate_error` has no README.**

  [`features/schedule/duplicate_error`](features/schedule/duplicate_error) violates the per-feature README requirement in [`README.md`](README.md#L217), so its intended behavior cannot be audited against a local specification.

  Acceptance criteria:

  - The directory contains a README with the feature purpose and exact expected behavior.
  - The Rust assertions are checked against that documented contract.

  Resolution: Implemented; manual review pending. Fix commit: `337bd1733b8fec33733e435124ddb774db8fc2fb`. Verification: `cargo fmt --check && cargo check --workspace`; `go run . run --lang rs --version 0.7.0 --no-history-check schedule/duplicate_error`.

## Verification record

The following checks were run during the audit:

| Check | Result |
| --- | --- |
| `cargo fmt --check` | Passed |
| `cargo clippy --workspace --all-targets -- -D warnings` | Passed |
| `cargo test --workspace` | Passed, but ran zero tests |
| `go test ./sdkbuild ./cmd` | Passed |
| Full live Rust default-feature suite on SDK 0.7 | 53 of 54 passed; `signal/prevent_close` failed |
| Worker shutdown, cancel-polls enabled variant | Passed |
| Worker shutdown, cancel-polls disabled variant | Passed |
| Targeted `update/validation_replay` live run | Passed without forcing replay, confirming the false-positive path |
| Targeted build/run on the CI-configured SDK 0.6.0 | Failed during dependency resolution |

## Resolution notes

When resolving a finding, add a short note beneath it containing:

- the fixing commit or pull request;
- the command used to verify the fix;
- any deliberate deviation from the README, including the README change that documents it.
