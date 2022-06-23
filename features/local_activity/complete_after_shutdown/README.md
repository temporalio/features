# Complete after shutdown

A Local Activity can complete after a Worker has initiated shutdown before shutdown has been finalized.

The Worker should be able to finalize shutdown once the Local Activity completes.

# Detailed spec
* SDKs should not prevent the completion of currently running local activities once
  shutdown has been requested
* If and when the local activity does finish, the workflow task should be completed
  as it normally would be including the appropriate `RecordMarker` command