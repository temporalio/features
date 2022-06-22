# Update a workflow from a gRPC client and poll for a response

In Temporal we can invoke a registered workflow update function and indicate
that the client will poll the server for the result of the workflow update at
some point in the future. The server will preserve the result of the update
function and return it as the result of the poll call.

# Detailed spec

The OSS server supports the `UPDATE_WORKFLOW_RESULT_ACCESS_STYLE_POLL`
value for the update request's `result_access_style`. When this value is
supplied, the server returns a result token to the gRPC client as soon as the
workflow has accepted (but not necessarily completed) the update. The result
token can be passed to future RPC calls to poll for the completed result of the
update.
