# Activity behavior during worker shutdown
Activities which are still running when a worker begins shutdown (ex: because ctrl-c was pressed)
are given a chance to shutdown. They are notified of this via a cancellation (which should indicate
it is a result of worker shutdown, rather than a real cancel), which they may choose to handle
however they desire.

The feature workflow should start a few activities and wait for their completion. Then the driver should
start worker shutdown. The activities should varyingly accept the cancel, complete successfully,
complete with failure, and ignore the cancel. The one which ignores the cancel should eventually
encounter a hard timeout.

# Detailed spec
* When a worker shutdown is initiated, the activity context isn't canceled until the
  graceful shutdown timeout has elapsed
* Graceful shutdown language behavior
  * Core based SDKs - when graceful shutdown timeout isn't specified, that is treated as no-timeout
  * Go - when graceful shutdown timeout isn't specified, this is treated as a 0 second timeout
  * Java - there is no timeout parameter set like Go or Core, but instead after calling `shutdown()` 
    a user can call `workerFactory.awaitTermination(timeout, unit)` and `isTerminated()` to see if the timeout
    has passed, then it is up to the user to forcibly shutdown or do something else
* It must be possible for an activity to determine that a worker is being shut down even before graceful timeout elapses
* It must be possible for the user to determine whether the activity context cancel was the result of worker shutdown,
  or issued by server
  * TODO: Java - need to add a way for Activities to know when the worker is being shutdown.
  * TODO: Typescript - figure out what happens in TS
* Activities may handle the cancel however they like (including continuing running)
* Heartbeating is possible while the shutdown process is ongoing
* If all activities complete, shutdown can complete (assuming WFT work is complete too)
* If an activity runs indefinitely and ignores the cancelation, then worker shutdown will hang indefinitely.
