# Rust SDK feature test failures

## update/async_accepted

`WorkflowUpdateHandle::get_result` does not report the expected gRPC `DeadlineExceeded` status for
the timeout in its `RpcOptions`. The test starts a two-second update and requests its result with a
200 ms RPC timeout; live runs either remained pending through a two-second watchdog or returned
`Cancelled` backed by a transport `TimeoutExpired` error.

## continue_as_new/updates_do_not_block_continue_as_new

Rust rejects `WorkflowHandle::start_update` when it is configured to wait for the `Admitted`
stage, returning `UpdateWorkflowExecution issued asynchronously and waiting on update admitted is
not supported.` The test requires that stage to synchronize an admitted update with an in-flight
workflow task before continuing as new, so it cannot perform the post-continue update assertions.

## update/task_failure

A panic in an update validator repeatedly fails workflow tasks instead of returning an error to
the update caller. The validator is invoked and panics again on each retry, so the test uses a
ten-second watchdog and reports `panicking validator repeatedly failed workflow tasks` rather than
waiting indefinitely; it cannot proceed to the separate update-execution panic assertions.

## Worker shutdown after feature completion

After successful feature assertions, invoking the Rust worker's shutdown handle does not reliably
cause `Worker::run()` to resolve. The future remained pending past a ten-second watchdog for
`continue_as_new/continue_as_same`, multiple data-converter tests, and deployment-versioning
workers, while other workers shut down promptly through the same path. The Rust harness reports
the stall and drops the run future after the watchdog so unrelated feature assertions can
continue.

