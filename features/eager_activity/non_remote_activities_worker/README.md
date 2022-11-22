# Eager activities with non-remote-activities worker

When an eager activity is scheduled by a workflow on a non-remote-activities worker, verify it does not get executed and the worker does not crash.

# Detailed spec

- Start a worker with activities registered and non-local activities disabled
- Run a workflow that schedules a single activity with short schedule-to-close timeout
- Catch activity failure in the workflow, check that it is caused by schedule-to-start timeout
