# Serialization context: failure conversion

Failures are converted with a serialization context too: activity failures with
the activity context, workflow failures with the workflow context.

Steps:

- register a failure converter that records the signature of its serialization
  context in `Failure.Source`
- run a workflow whose activity fails, and which then fails itself
- verify the client sees the workflow error
- verify that `ActivityTaskFailed` carries the activity signature
- verify that `WorkflowExecutionFailed` carries the workflow signature

Python only puts an activity ID in the workflow side context when the
workflow sets one explicitly, so the workflow schedules the activity with an
explicit activity ID.
