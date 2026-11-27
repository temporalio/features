# Rejected Update followed by worker restart

A Workflow with no pending work rejects an Update in a speculative Workflow Task.
That rejection must leave the persisted history unchanged. After the worker is
replaced, an accepted Update must execute once and the Workflow must replay.

This combines nondurable Update rejection with cache loss. The existing
`non_durable_reject` feature keeps using the same worker, while `worker_restart`
restarts during an accepted Update. The test waits for the initial Workflow Task
to finish, compares the complete history before and after rejection, restarts
the worker, then checks the accepted Update result, history, and replay.
