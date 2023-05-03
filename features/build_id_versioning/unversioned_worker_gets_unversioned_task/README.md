# Build ID Versioning: Unversioned Workers get Unversioned Tasks

If an unversioned worker starts up targeting a versioned task queue, it should only receive
tasks for workflows that have no assigned version

# Detailed spec

* Start a workflow on an unversioned task queue
* Start a workflow
* Add a version to the queue
* Start up an unversioned worker, verify it can process tasks for the unversioned workflow
