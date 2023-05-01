# Build ID Versioning: Activity and Child Workflows on correct version

If a workflow starts an activity or a child workflow on its own (versioned) task queue, then by
default those tasks should run on the same compatible set as the launching workflow. The user may
opt to have them run on the queue's overall default if they choose.

If a workflow starts an activity or child wf on a different task queue, then that task runs on that
queue's default version (if it has one - or no versioning at all).

# Detailed spec

* Create versioned task queue which has version sets `{1.0}` 
* Start a `1.0` worker
* Start a `2.1` worker
* Start the workflow and it should wait for signal to proceed
* Add version `{2.0}` to the queue
* Signal the workflow to proceed
* Run an activity & child wf w/ default options - they must complete on the `1.0` worker
* Run an activity & child wf w/ option to use default version - it must complete on the `2.1` worker
