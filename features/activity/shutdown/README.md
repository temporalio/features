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
* When a worker shutdown is initiated, the activity context isn't canceled until shutdown
  has succeeded or the shutdown timeout has elapsed
* It must be possible for the user to determine if this cancel was issued by server,
  or is the result of worker shutdown
  * TODO: Java - need to add a way for Activities to know when the worker is being shutdown.
  * TODO: Typescript - figure out what happens in TS
* Activities may handle the cancel however they like (including continuing running)
* Heartbeating is possible while the shutdown process is ongoing
* If all activities complete, shutdown can complete (assuming WFT work is complete too)
* For Core based SDKs, if an activity runs indefinitely and ignores the cancelation, then worker shutdown will
  hang indefinitely.
  * TODO: verify Java
