# Local Activity gets cancelled when a Worker is shutting down

When a Worker is shutting down, it cancels any running Local Activities.

# Detailed spec

* When an SDK begins shutdown, for non-Abandon mode LAs, they will be notified of the cancel
* Shutdown will not complete until all LAs have completed, either by acknowledging the cancel,
  succeeding, or failing
* If any LAs acknowledged the cancel, *no* marker is recorded for them. They will start from
  attempt 0 the next time the workflow is invoked. In such a situation, the SDK must force a 
  new workflow task (but not request it to be delivered back) to ensure that the workflow does
  not get stuck.
* If any LAs succeeded or failed completely (no more retries), markers *are* recorded for them.