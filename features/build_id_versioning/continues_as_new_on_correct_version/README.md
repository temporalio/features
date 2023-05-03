# Build ID Versioning: Continues As New On Correct Version

If a workflow continues as new on its own (versioned) task queue, then by default the new workflow
should run on the same compatible set as the continued workflow. The user may opt to have it run on
the queue's overall default if they choose.

If a workflow starts continues as new on a different task queue, then that task runs on that
queue's default version (if it has one - or no versioning at all).

# Detailed spec

* Create versioned task queue which has version sets `{1.0}` 
* Start a `1.0` worker
* Start a `2.0` worker
* Start the workflow and it should wait for a signal to proceed
* Add version `{2.0}` to the queue
* Signal the workflow to proceed, and it should continue as new
* See that the continued workflow started on the `1.0` worker
* Tell it to continue again, this time using the overall default, see it run on the `2.0` worker
