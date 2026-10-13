# Signal with start from a workflow

A workflow starts or signals another workflow with the reserved System Nexus
`SignalWithStartWorkflowExecution` operation. The first call starts the target
and delivers a signal; the second call signals the existing target execution.
Both workflows run in the same namespace.

System Nexus routes this operation through endpoint `__temporal_system`,
service `temporal.api.workflowservice.v1.WorkflowService`, and operation
`SignalWithStartWorkflowExecution`. The endpoint supports targets in the
caller's namespace only.

## Shared contract

- Both calls use workflow ID conflict policy `USE_EXISTING`.
- The first call supplies `start-value` as the target start input and sends
  `signal-one`.
- The second call supplies a deliberately unused `unused-start-value` and
  sends `signal-two`.
- Both calls address the same target workflow run.
- The embedded dev server enables this feature with
  `history.enableSignalWithStartFromWorkflow=true`.

## Language coverage

| Language | API surface | SDK requirement | Assertions |
|---|---|---|---|
| Python | `workflow.signal_with_start_workflow` | >= 1.29.0 | Both calls return the same target run ID; the target returns `started: start-value`, `signal: signal-one`, `signal: signal-two` in order, proving the second start input was ignored. |

## Scope boundary

This feature does not cover custom payload converters, codecs, or external
storage. Those components are reserved for a follow-up feature.
