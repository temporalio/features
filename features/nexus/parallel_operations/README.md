# Nexus operations run in parallel

A workflow starts three synchronous Nexus operations in a single workflow task and awaits all
of their results.

# Detailed spec

- All three operations are started before any of them is awaited, so they are scheduled in the
  same workflow task.
- The caller awaits the operation futures in order and joins their results.
- The history contains one scheduled and one completed event per operation and no started
  events, since sync operations never enter the started state.
