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
* If a worker is told to begin shutdown, activities are immediately notified via
  a cancel. It must be possible for the user to determine if this cancel was issued
  by server, or is the result of worker shutdown
* Activities may handle the cancel however they like (including continuing running)
* Heartbeating is possible while the shutdown process is ongoing
* If all activities complete, shutdown can complete (assuming WFT work is complete too)
* SDKs should provide a timeout parameter which, if elapsed, shutdown completes even if
  there are still running activities. If such activities complete later their response
  is ignored
