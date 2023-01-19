# Updates Survive Worker Restarts

If the worker running an update restarts or otherwise loses its workflow cache
while executing an update then the update will be replayed and resumed once the
worker comes back.

# Detailed spec

- Start an update that intentionally does a bad thing in blocking on something
  controllable by the test case, e.g. a latch.
- Stop the worker.
- Adjust the blocking mechanism to allow the update to procede (e.g. close the
  latch)
- Start the worker.
- Observe that the update runs to completion.

