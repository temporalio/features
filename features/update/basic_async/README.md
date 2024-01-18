# Update a workflow

A Workflow Update can be performed using the "async" API.

# Detailed spec

An Update can be defined, the handler and validator can be set.
An initial request is made to start an update; this request blocks until the update is Accepted or Rejected, and returns an update handle.
The update handle is then used to retrieve the result (this request blocks until completed).
If the validator rejects, then issuing the update results in a failure.
Alternatively if the validator accepts, then issuing the update results in both the expected mutation and the expected return value.
