# Nexus async operation is cancelled

A workflow cancels a running asynchronous Nexus operation and observes a cancellation error.

# Detailed spec

- The backing workflow of the operation blocks until it is cancelled, so the cancellation is
  deterministic and never races with completion.
- The caller starts the operation in a cancellable scope and waits until the operation has
  actually started before cancelling that scope.
- Cancelling the scope requests cancellation of the operation, which cancels the backing
  workflow, and the operation future resolves with a cancellation error.
- The caller explicitly waits for operation completion on cancellation. The result is a Nexus
  operation error with the expected endpoint, service, operation, positive scheduled event ID,
  and the same non-empty token returned by the operation execution.
- Its immediate cause is a leaf cancellation error without details, rather than merely
  containing cancellation somewhere in an otherwise incorrect cause chain.
- The caller handles that error and completes successfully.
- The history records exactly scheduled, started, cancel requested, and canceled lifecycle
  events, in that order, with no failed, completed, or timed-out operation. A successful
  cancellation-request acknowledgement is allowed because it can race with operation
  cancellation. The caller's history is replayed.

Other cancellation modes have independently runnable features:

- [`async_cancellation_try_cancel`](../async_cancellation_try_cancel/README.md) checks return
  before cancellation delivery.
- [`async_cancellation_wait_requested`](../async_cancellation_wait_requested/README.md) checks
  waiting for acknowledgement but not operation completion.
- [`async_cancellation_abandon`](../async_cancellation_abandon/README.md) checks local cancellation
  without canceling the operation.
