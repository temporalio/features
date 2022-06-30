# One Local Activity succeeds one Local Activity fails in the same Workflow Task

When two (or more) LAs are running concurrently and some of them succeed or some of them fail,
the SDK will persist the successes, but not the failures (unless those failures were the final
attempt).

# Detailed spec

* If any LAs failed but have more attempts remaining, *no* marker is recorded for them. In such a
  situation, the SDK must force a new workflow task.
* If any LAs succeeded or failed completely (no more retries), markers *are* recorded for them.
* If the worker crashes or evicts the workflow, not-completely-failed LAs will restart from attempt
  0.