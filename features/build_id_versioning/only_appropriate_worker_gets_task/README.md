# Build ID Versioning: Only Appropriate Worker Gets Task

A workflow which is started on a worker with a certain version should only continue to execute on
worker(s) with a version belonging to that compatible set.

Workers with no or incompatible versions should never receive tasks for the workflow.

# Detailed spec

* Start a workflow on a versioned task queue which has version sets `{1.0}, {2.0, 2.1}`
* Verify the workflow begins executing when a `2.1` worker is live
* Stop the worker before the workflow ends
* Start `1.0` and `2.0` workers, verify they do not process any tasks, stop them
* Start a `2.1` worker again, verify it completes the workflow