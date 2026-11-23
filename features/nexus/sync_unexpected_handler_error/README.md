# Nexus unexpected handler error becomes retryable

A workflow invokes a synchronous Nexus handler that returns an ordinary unexpected error,
rather than explicitly constructing a Nexus handler error or an operation error.

# Detailed spec

- Go returns an ordinary error; Java throws an ordinary runtime exception.
- An atomic counter makes the first handler call fail and the next call succeed.
- After the workflow completes, the harness checks the successful result and exactly
  two handler calls: one failure followed by one successful retry.
- This verifies retryable handling of an unexpected error through workflow-visible
  behavior. Intermediate failure classification, internal messages, and causes are not
  inspected; there are no Describe calls or polling loops.
- The caller receives the successful result, not an intermediate operation failure.
- History contains exactly one scheduled and one completed Nexus event, with no started
  or failed event and no additional scheduled operation.
- The counter is incremented only by the handler and checked after completion, not read
  by workflow code. The caller's history is replayed.
