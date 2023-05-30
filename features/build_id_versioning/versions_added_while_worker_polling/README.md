# Build ID Versioning: Versions added while worker polling

If a version is added to a queue which already has versions while a worker which would be compatible
with that new version is polling, it should receive tasks for that version.

If a version is added to a set as default while worker(s) with existing versions in that set are
polling, they should not receive newly created tasks, since those tasks must be handled by the newly
added default.

# Detailed spec

* Start a workflow on a versioned task queue with a `1.0` version
* Complete 1 or more WFTs @ `1.0`
* Stop the worker
* Start a `1.1` worker
* See that it does not process tasks
* Add `1.1` to the sets
* See that it starts processing tasks
* Add `1.2` to the sets
* See that it no longer processes tasks
