# Activity Start Potential Race Condition

We were seeing an error where a history had the following events in order:

* `WorkflowTaskScheduled`
* `ActivityTaskStarted`
* `ActivityTaskCompleted`
* `WorkflowTaskStarted`

This was an attempt to replicate a user bug as part of [this issue](https://github.com/temporalio/sdk-go/issues/670).
The [history/history.manual.json](history/history.manual.json) contains a history of a run of this workflow that is
tested via replay. The panic caused in the issue could not be replicated and the history that is generated from the
workflow is subject to non-deterministic external timing so it doesn't replicate the exact same steps in order each
time. This is why the history was captured to test replays against.