# Build ID Versioning: Unversioned Worker Does Not Get Task

If an unversioned worker starts up targeting a versioned task queue, it should not receive any tasks

# Detailed spec

* Start a workflow on a versioned task queue which has at least one version in it
* Start up an unversioned worker, verify it does not get a task
* Finish the workflow with an appropriately versioned worker
