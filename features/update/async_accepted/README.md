# Async Updates

Updates can be invoked asynchronously by waiting on the ACCEPTED state.

# Detailed spec

It will be possible to invoke an update and indicate that the calling RPC client
wants the RPC call to block on on completion of the update but rather on the the
update passing validation on the workflow - this is called the "Accepted" state.
The update continues to execute to completion despite there being no caller
waiting on its outcome.

A client with knowledge of the update's identifying information (viz.,
workflow ID, run ID, and update ID) can create a handle to that update and await
the outcome of the update call. Outcomes returned from these asynchronous
updates are the same as those that would have been returned inline had the
update been invoked synchronously.

