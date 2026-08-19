# Nexus async operation is cancelled

A workflow cancels a running asynchronous Nexus operation and observes a cancellation error.

# Detailed spec

- The backing workflow of the operation blocks until it is cancelled, so the cancellation is
  deterministic and never races with completion.
- The caller starts the operation in a cancellable scope and waits until the operation has
  actually started before cancelling that scope.
- Cancelling the scope requests cancellation of the operation, which cancels the backing
  workflow, and the operation future resolves with a cancellation error.
- The caller handles that error and completes successfully.
