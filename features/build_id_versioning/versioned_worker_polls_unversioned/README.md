# Build ID Versioning: Versioned Worker Polls Unversioned Queue

If a versioned worker starts up targeting an unversioned task queue, it should get an error when 
polling.

# Detailed spec

* Start a versioned worker against a task queue with no versions, it should error out
